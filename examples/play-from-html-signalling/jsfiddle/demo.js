/* eslint-env browser */
let signallingServer = "http://hyperscale-stg.coldsnow.net:9091";
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

pc.oniceconnectionstatechange = e => {
  log(pc.iceConnectionState);
  if (pc.iceConnectionState == "connected") {
    const localVideo = document.getElementById('video1');
    //localVideo.src = "http://hyperscale.coldsnow.net:8080/hyperscale.mp4";
    localVideo.play();
  }
}
pc.icecandidateerror = event => log(event);
pc.ondatachannel = e => {
  let dc = e.channel
  log('new DataChannel ' + dc.label + ' was created.')
  dc.onclose = () => console.log('datachannel has closed')
  dc.onopen = () => console.log('datachannel has opened')
  dc.onmessage = e => {
    console.log(`Message from DataChannel '${dc.label}' payload '${e.data}'`)
    log(`>> got message: ${e.data}`);
  }

  window.sendMessage = () => {
    let message = document.getElementById('message').value
    if (message === '') {
      return alert('Message must not be empty')
    }
    dc.send(message)
  }
}

let connectionId;

pc.onicecandidate = async event => {
  if (event.candidate === null) {
    await sendAnswer(connectionId, pc.localDescription.toJSON());
  }
}

async function getConnection() {
  try{
    console.log("trying to get available offer..");

    let response = await fetch(`${signallingServer}/signaling/1.0/application/queue`, {
      method: 'get'
    })

    if (response.ok){
      let body = await response.json();
      connectionId = body.connectionId;
      getOffer(connectionId);
      return;
    }


    setTimeout(() => getConnection(), 1000)

  } catch (e) {
    alert(e);
  }
}
async function getOffer(connectionId) {
  try{

    console.log(`trying to get connection ${connectionId} information..`);

    let response = await fetch(`${signallingServer}/signaling/1.0/connections/${connectionId}/offer`, {
      method: 'get'
    })

    let body = await response.json();

    if (response.ok && body != ""){
      delete body.deviceId;
      await pc.setRemoteDescription(new RTCSessionDescription(body));

      const localVideo = document.getElementById('video1');
      let localStream = localVideo.captureStream();
      localStream.getTracks().forEach(track => pc.addTrack(track, localStream));

      let answer = await pc.createAnswer();
      await pc.setLocalDescription(new RTCSessionDescription(answer));
      return;
    }

    setTimeout(() => getOffer(), 1000)

  } catch (e) {
    alert(e);
  }
}
async function sendAnswer(connectionId, answer) {
  try {
    // send answer to signalling server
    let response =  await fetch(`${signallingServer}/signaling/1.0/connections/${connectionId}/answer`, {
      method: 'post',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify(answer)
    })

    console.log("posted an answer..");

    await response.json();

  } catch(e) {
    alert(e);
    return null;
  }
}

getConnection();
