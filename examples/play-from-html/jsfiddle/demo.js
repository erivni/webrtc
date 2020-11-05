/* eslint-env browser */

let pc = new RTCPeerConnection({
  iceServers: [
    {
      urls: 'stun:stun.l.google.com:19302'
    }
  ]
})
var log = msg => {
  document.getElementById('logs').innerHTML += msg + '<br>'
}

pc.oniceconnectionstatechange = e => log(pc.iceConnectionState);

pc.onicecandidate = event => {
  if (event.candidate === null) {
    document.getElementById('localSessionDescription').value = btoa(JSON.stringify(pc.localDescription))
  }
}

pc.icecandidateerror = event => log(event);

window.startSession = async () => {
  let sd = document.getElementById('remoteSessionDescription').value
  if (sd === '') {
    return alert('Session Description must not be empty')
  }

  try {
    await pc.setRemoteDescription(new RTCSessionDescription(JSON.parse(atob(sd))))

    const localVideo = document.getElementById('video1');
    let localStream = localVideo.captureStream();
    localStream.getTracks().forEach(track => pc.addTrack(track, localStream));

    let answer = await pc.createAnswer();
    await pc.setLocalDescription(new RTCSessionDescription(answer));

  } catch (e) {
    alert(e)
  }
}
