# play-h264-from-gstreamer
play-h264-from-gstreamer demonstrates how to send h264 video to your browser from a gstreamer pipeline.


## UI RTP gstreamer pipeline examples
#### Linux
ximagesrc: \
`gst-launch-1.0 -v ximagesrc xname="Clock" show-pointer="false" ! videoscale ! video/x-raw,framerate=25/1,width=640,height=360 ! videoscale ! videoconvert ! video/x-raw, format=I420 ! x264enc key-int-max=25 tune=zerolatency bitrate=500 speed-preset=superfast ! rtph264pay ! udpsink host=127.0.0.1 port=5000`

videotestsrc: \
`gst-launch-1.0 -v videotestsrc ! timeoverlay ! videoscale ! video/x-raw,framerate=25/1,width=640,height=360 ! videoscale ! videoconvert ! video/x-raw, format=I420 ! x264enc key-int-max=25 tune=zerolatency bitrate=500 speed-preset=superfast ! rtph264pay ! udpsink host=127.0.0.1 port=5000`


#### macOS
avfvideosrc: \
`gst-launch-1.0 -v avfvideosrc capture-screen=true ! videoscale ! video/x-raw,framerate=25/1,width=640,height=360 ! videoscale ! videoconvert ! video/x-raw, format=I420 ! x264enc key-int-max=25 tune=zerolatency bitrate=500 speed-preset=superfast ! rtph264pay ! udpsink host=127.0.0.1 port=5000`

videotestsrc: \
`gst-launch-1.0 -v videotestsrc ! timeoverlay ! videoscale ! video/x-raw,framerate=25/1,width=640,height=360 ! videoscale ! videoconvert ! video/x-raw, format=I420 ! x264enc key-int-max=25 tune=zerolatency bitrate=500 speed-preset=superfast ! rtph264pay ! udpsink host=127.0.0.1 port=5000`

Congrats, you have used Pion WebRTC! Now start building something cool
