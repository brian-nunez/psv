package psv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ErrNoVideo is returned for a file without a video stream.
var ErrNoVideo = errors.New("psv: no video stream")

// Tools locates the FFmpeg executables. Empty fields are looked up on PATH.
type Tools struct {
	FFmpeg  string
	FFprobe string
}

func (t Tools) ffmpeg() string {
	if t.FFmpeg != "" {
		return t.FFmpeg
	}

	return "ffmpeg"
}

func (t Tools) ffprobe() string {
	if t.FFprobe != "" {
		return t.FFprobe
	}

	return "ffprobe"
}

// Rational is an exact frame rate.
type Rational struct {
	Num, Den int
}

// Float64 returns the rate as a number, or zero when it is undefined.
func (r Rational) Float64() float64 {
	if r.Den == 0 {
		return 0
	}

	return float64(r.Num) / float64(r.Den)
}

func (r Rational) String() string {
	return fmt.Sprintf("%d/%d", r.Num, r.Den)
}

func parseRational(s string) Rational {
	num, den, ok := strings.Cut(s, "/")
	if !ok {
		den = "1"
	}

	n, err1 := strconv.Atoi(num)
	d, err2 := strconv.Atoi(den)

	if err1 != nil || err2 != nil || n <= 0 || d <= 0 {
		return Rational{}
	}

	return Rational{Num: n, Den: d}
}

// VideoInfo describes the first video stream of a file.
type VideoInfo struct {
	Width, Height int
	// FrameRate is the stream's average frame rate, or its nominal rate when
	// the average is unknown.
	FrameRate Rational
	// Frames is the number of frames the container declares, or zero when it
	// does not declare one.
	Frames   int
	Duration time.Duration
	Codec    string
}

// Probe reads the description of the first video stream of path.
func Probe(ctx context.Context, path string, tools Tools) (VideoInfo, error) {
	cmd := exec.CommandContext(ctx, tools.ffprobe(),
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_name,width,height,avg_frame_rate,r_frame_rate,nb_frames,duration:format=duration",
		"-of", "json",
		path,
	)

	var stderr bytes.Buffer

	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return VideoInfo{}, toolError("ffprobe", err, &stderr)
	}

	var doc struct {
		Streams []struct {
			CodecName    string `json:"codec_name"`
			Width        int    `json:"width"`
			Height       int    `json:"height"`
			AvgFrameRate string `json:"avg_frame_rate"`
			RFrameRate   string `json:"r_frame_rate"`
			NbFrames     string `json:"nb_frames"`
			Duration     string `json:"duration"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}

	if err := json.Unmarshal(out, &doc); err != nil {
		return VideoInfo{}, fmt.Errorf("psv: parse ffprobe output: %w", err)
	}

	if len(doc.Streams) == 0 {
		return VideoInfo{}, ErrNoVideo
	}

	s := doc.Streams[0]
	info := VideoInfo{
		Width:     s.Width,
		Height:    s.Height,
		FrameRate: parseRational(s.AvgFrameRate),
		Codec:     s.CodecName,
	}

	if info.FrameRate.Den == 0 {
		info.FrameRate = parseRational(s.RFrameRate)
	}

	if n, err := strconv.Atoi(s.NbFrames); err == nil {
		info.Frames = n
	}

	for _, d := range []string{s.Duration, doc.Format.Duration} {
		if sec, err := strconv.ParseFloat(d, 64); err == nil && sec > 0 {
			info.Duration = time.Duration(sec * float64(time.Second))

			break
		}
	}

	if info.Width <= 0 || info.Height <= 0 {
		return VideoInfo{}, fmt.Errorf("psv: invalid frame size %dx%d", info.Width, info.Height)
	}

	return info, nil
}

// DecodeOptions controls decoding.
type DecodeOptions struct {
	// FrameRate, when nonzero, converts the video to this constant frame
	// rate by repeating or dropping frames. Otherwise every frame is decoded
	// once, in presentation order.
	FrameRate int
	// Deinterlace deinterlaces frames that are flagged as interlaced.
	Deinterlace bool
}

// prefetch is the number of frames decoded ahead of the one in use.
const prefetch = 2

// Decoder streams the frames of a video. It decodes ahead of the caller, so
// that decoding overlaps analysis.
type Decoder struct {
	cmd    *exec.Cmd
	out    io.ReadCloser
	stderr bytes.Buffer

	// free holds frame buffers ready to be filled and ready the decoded
	// frames, followed by the read error that ends the stream.
	free  chan *Frame
	ready chan decoded
	stop  chan struct{}

	held   *Frame
	err    error
	waited bool
	closed bool
}

type decoded struct {
	frame *Frame
	err   error
}

// Decode starts decoding the first video stream of path. info must describe
// the file, as returned by Probe.
func Decode(ctx context.Context, path string, info VideoInfo, tools Tools, opts DecodeOptions) (*Decoder, error) {
	filter := "scale=in_color_matrix=auto:in_range=auto:out_range=pc:flags=accurate_rnd+full_chroma_int,format=rgb24"
	if opts.FrameRate > 0 {
		filter = fmt.Sprintf("fps=%d,%s", opts.FrameRate, filter)
	}

	if opts.Deinterlace {
		filter = "yadif=deint=interlaced," + filter
	}

	d := &Decoder{
		free:  make(chan *Frame, prefetch+1),
		ready: make(chan decoded, prefetch+2),
		stop:  make(chan struct{}),
	}

	for range prefetch + 1 {
		d.free <- &Frame{Width: info.Width, Height: info.Height, Pix: make([]byte, info.Width*info.Height*3)}
	}

	d.cmd = exec.CommandContext(ctx, tools.ffmpeg(),
		"-nostdin", "-hide_banner", "-loglevel", "error",
		"-noautorotate",
		"-i", path,
		"-map", "0:v:0", "-an", "-sn", "-dn",
		"-vf", filter,
		"-fps_mode", "passthrough",
		"-f", "rawvideo", "-pix_fmt", "rgb24",
		"pipe:1",
	)
	d.cmd.Stderr = &d.stderr

	out, err := d.cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("psv: %w", err)
	}

	d.out = out

	if err := d.cmd.Start(); err != nil {
		return nil, fmt.Errorf("psv: start ffmpeg: %w", err)
	}

	go d.read()

	return d, nil
}

// read decodes frames into free buffers until the stream ends or the
// decoder is closed.
func (d *Decoder) read() {
	defer close(d.ready)

	for {
		var f *Frame

		select {
		case f = <-d.free:
		case <-d.stop:
			return
		}

		if _, err := io.ReadFull(d.out, f.Pix); err != nil {
			d.ready <- decoded{err: err}

			return
		}

		d.ready <- decoded{frame: f}
	}
}

// Next returns the next frame, or io.EOF after the last one. The frame is
// valid until the following call.
func (d *Decoder) Next() (*Frame, error) {
	if d.held != nil {
		d.free <- d.held

		d.held = nil
	}

	if d.err != nil {
		return nil, d.err
	}

	r, ok := <-d.ready
	if !ok {
		d.err = io.EOF

		return nil, d.err
	}

	if r.err == nil {
		d.held = r.frame

		return r.frame, nil
	}

	d.err = io.EOF
	if werr := d.wait(); werr != nil {
		d.err = werr
	} else if !errors.Is(r.err, io.EOF) {
		d.err = fmt.Errorf("psv: truncated frame: %w", r.err)
	}

	return nil, d.err
}

// Close stops decoding and releases the decoder.
func (d *Decoder) Close() error {
	if d.closed {
		return nil
	}

	d.closed = true
	close(d.stop)

	err := d.out.Close()

	for range d.ready {
	}

	if !d.waited {
		// Closing the pipe ends ffmpeg early; its exit status is not an error.
		_ = d.cmd.Wait()
		d.waited = true
	}

	if err != nil && !errors.Is(err, os.ErrClosed) {
		return fmt.Errorf("psv: %w", err)
	}

	return nil
}

func (d *Decoder) wait() error {
	d.waited = true

	if err := d.cmd.Wait(); err != nil {
		return toolError("ffmpeg", err, &d.stderr)
	}

	return nil
}

func toolError(tool string, err error, stderr *bytes.Buffer) error {
	if msg := strings.TrimSpace(stderr.String()); msg != "" {
		return fmt.Errorf("psv: %s: %s: %w", tool, msg, err)
	}

	return fmt.Errorf("psv: %s: %w", tool, err)
}
