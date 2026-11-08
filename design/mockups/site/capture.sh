#!/bin/sh
# Render each example report with psv render and capture the report-UI previews
# that site.py places in the example cards. No video is attached to any report.
#
#   design/mockups/site/capture.sh
#
# Needs bin/psv (make build), Microsoft Edge and ffmpeg. Rendered reports go to
# reports/ and raw screenshots to .tmp/ (both gitignored); crops go to img/.
set -eu

here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../../.." && pwd)
psv=${PSV:-$root/bin/psv}
edge=${EDGE:-"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge"}

mkdir -p "$here/reports" "$here/.tmp/shots" "$here/.tmp/edge" "$here/img"

# shot <html> <width,height> <out.png>: a 2x headless screenshot of the page top.
shot() {
	"$edge" --headless=new --disable-gpu --hide-scrollbars --user-data-dir="$here/.tmp/edge" \
		--force-device-scale-factor=2 --window-size="$2" --screenshot="$3" "file://$1" >/dev/null 2>&1
}

# crop <in> <w:h:x:y> <out>
crop() {
	ffmpeg -loglevel error -y -i "$1" -vf "crop=$2" "$3"
}

for name in electric-soldier-porygon-strobes colors-epilepsy-warning psychedelic-flash-short video-ad-sample-2; do
	html="$here/reports/$name.html"
	"$psv" render -o "$html" "$root/testdata/examples/$name/report.json"

	# Phone cards: the report at 560 px, from the file name to the overview.
	shot "$html" 560,1400 "$here/.tmp/shots/$name-560.png"
	crop "$here/.tmp/shots/$name-560.png" 1088:1340:16:200 "$here/img/$name-phone.png"

	if [ "$name" = electric-soldier-porygon-strobes ]; then
		# Desktop featured card: the report at 1440 px, to the first moment's chart.
		shot "$html" 1440,1100 "$here/.tmp/shots/$name-1440.png"
		crop "$here/.tmp/shots/$name-1440.png" 2240:1688:320:192 "$here/img/$name-featured.png"
	else
		# Desktop row cards: the report at 960 px, from the file name to the overview.
		shot "$html" 960,1400 "$here/.tmp/shots/$name-960.png"
		crop "$here/.tmp/shots/$name-960.png" 1920:1400:0:180 "$here/img/$name-row.png"
	fi
	echo "captured $name"
done
