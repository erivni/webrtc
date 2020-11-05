/* eslint-env browser */
let pc = new RTCPeerConnection({
  iceServers: [
    {
      urls: 'stun:stun.l.google.com:19302'
    }
  ]
})
let log = msg => {
  document.getElementById('div').innerHTML += msg + '<br>'
}
const dataChannelOptions = {
  ordered: true, // do not guarantee order
  maxPacketLifeTime: 3000, // in milliseconds
};
let sendChannel = pc.createDataChannel('hyperscale', dataChannelOptions);
sendChannel.onclose = () => log('sendChannel has closed');
sendChannel.onopen = () => log('sendChannel has opened');
sendChannel.onmessage = e => log(`Message from DataChannel '${sendChannel.label}' payload '${e.data}'`);
pc.ontrack = function (event) {
  if (event.track.kind === 'audio') {
    return
  }
  var el = document.createElement(event.track.kind)
  el.srcObject = event.streams[0]
  el.autoplay = true
  el.controls = true
  document.getElementById('remoteVideos').appendChild(el)
}
pc.oniceconnectionstatechange = e => log(pc.iceConnectionState)
pc.onicecandidate = event => {
  if (event.candidate === null) {
    document.getElementById('localSessionDescription').value = btoa(JSON.stringify(pc.localDescription))
  }
}
// Offer to receive 1 audio, and 1 video track
pc.addTransceiver('video', {'direction': 'sendrecv'})
pc.addTransceiver('audio', {'direction': 'sendrecv'})
pc.createOffer().then(d => pc.setLocalDescription(d)).catch(log)
window.startSession = () => {
  let sd = document.getElementById('remoteSessionDescription').value
  if (sd === '') {
    return alert('Session Description must not be empty')
  }
  try {
    pc.setRemoteDescription(new RTCSessionDescription(JSON.parse(atob(sd))))
  } catch (e) {
    alert(e)
  }
}
window.startUI = async () => {
  try {
    let msg = {
      type: "switch",
      target: "ui"
    }
    sendChannel.send("ui");
    /*
    fetch("http://localhost:8011/switch", {
      method: 'post',
      body: "ui"
    })
     */
    log("switched to ui")
  } catch (e) {
    alert(e)
  }
}
window.startABR = async () => {
  try {
    let msg = {
      type: "switch",
      target: "abr"
    }
    sendChannel.send("abr");
    /*
    fetch("http://localhost:8011/switch", {
      method: 'post',
      body: "abr"
    })
     */
    log("switched to abr")
  } catch (e) {
    alert(e)
  }
}
