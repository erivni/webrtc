/* eslint-env browser */
let signallingServer = "http://hyperscale-stg.coldsnow.net:9091";
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
sendChannel.onmessage = e => {
  console.log(`Message from DataChannel '${sendChannel.label}' payload '${e.data}'`)
  log(`>> got message: ${e.data}`);
}
pc.ontrack = function (event) {
  if (event.track.kind === 'audio') {
    return
  }
  var el = document.createElement(event.track.kind)
  el.srcObject = event.streams[0]
  el.muted = true
  el.autoplay = true
  el.controls = true
  document.getElementById('remoteVideos').appendChild(el)
}
pc.oniceconnectionstatechange = e => log(pc.iceConnectionState);

pc.onicecandidate = async event => {
  if (event.candidate === null) {
    let offer = Object.assign({}, pc.localDescription.toJSON());
    offer.deviceId = makeid(5);

    let connectionId = await sendOffer(offer);
    if (connectionId !== null) {
      // print it out for debugging
      //document.getElementById('localSessionDescription').value = JSON.stringify(offer)
      getAnswer(connectionId);
    }
  }
}


// Offer to receive 1 audio, and 1 video track
pc.addTransceiver('video', {'direction': 'sendrecv'})
pc.addTransceiver('audio', {'direction': 'sendrecv'})
pc.createOffer().then(d => pc.setLocalDescription(d)).catch(log)

window.startUI = async () => {
  try {
    sendChannel.send("ui");
    log("switched to ui")
  } catch (e) {
    alert(e)
  }
}
window.startABR = async () => {
  try {
    sendChannel.send("abr");
    log("switched to abr")
  } catch (e) {
    alert(e)
  }
}

function makeid(length) {
  var result           = '';
  var characters       = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
  var charactersLength = characters.length;
  for ( var i = 0; i < length; i++ ) {
    result += characters.charAt(Math.floor(Math.random() * charactersLength));
  }
  return result;
}
async function sendOffer(offer) {
  try {
    // send offer to signalling server
    let response =  await fetch(`${signallingServer}/signaling/1.0/client/connections`, {
      method: 'post',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify(offer)
    })

    console.log("posted an offer..");

    let connection = await response.json();
    console.log("connection id is " + connection.connectionId)
    return connection.connectionId;
  } catch(e) {
    alert(e);
    return null;
  }
}
async function getAnswer(connectionId) {
  try{
    console.log("trying to get answer..");

    let response = await fetch(`${signallingServer}/signaling/1.0/connections/${connectionId}/answer`, {
      method: 'get'
    })

    let body = await response.text();

    if (response.ok && body != ""){
      //document.getElementById('remoteSessionDescription').value = body
      pc.setRemoteDescription(new RTCSessionDescription(JSON.parse(body)))
      return;
    }

    setTimeout(() => getAnswer(connectionId), 1000)

  } catch (e) {
    alert(e);
  }
}

