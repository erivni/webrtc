FROM golang:1.15 as builder

WORKDIR  /home/src/webrtc

RUN apt-get update && \
    apt-get install -y libgstreamer1.0-0 gstreamer1.0-plugins-base gstreamer1.0-plugins-good gstreamer1.0-plugins-bad gstreamer1.0-plugins-ugly gstreamer1.0-libav gstreamer1.0-doc gstreamer1.0-tools gstreamer1.0-x gstreamer1.0-alsa gstreamer1.0-gl gstreamer1.0-gtk3 gstreamer1.0-qt5 gstreamer1.0-pulseaudio libgstreamer-plugins-base1.0-dev

ENV PKG_CONFIG_PATH="/usr/lib/x86_64-linux-gnu/pkgconfig"

COPY . .

RUN apt-get install -y net-tools && \
    go build -o transcontainer -v ./examples/play-h264-from-gstreamer

CMD ["/home/src/webrtc/transcontainer"]

