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

  let innerHtml = document.getElementById('logs').innerHTML;
  let count = (innerHtml.match(/<br>/g) || []).length;

  if (count >= 15) { // rolling logs
    innerHtml = innerHtml.substr(4); // ignore first <br>
    let secondBr = innerHtml.indexOf("<br>");
    innerHtml = innerHtml.substr(secondBr);

    document.getElementById('logs').innerHTML =  innerHtml;
  }

  let now = new Date(Date.now()).toLocaleString();
  document.getElementById('logs').innerHTML += `${now}: ${msg}<br>`
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

  window.startUI = async () => {
    try {
      dc.send("ui");
      log("sent command to switch to ui")
    } catch (e) {
      alert(e)
    }
  }
  window.startABR = async () => {
    try {
      dc.send("abr");
      log("sent command to switch to abr")
    } catch (e) {
      alert(e)
    }
  }

  window.tuneTo = async () => {
    try {
      const channels = document.getElementById('channels');
      let selected_channel = channels.options[channels.selectedIndex].text;

      dc.send(`tuneTo: ${selected_channel}`);
      log(`sent command to tune to ${selected_channel}`)
    } catch (e) {
      alert(e)
    }
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

window.onload = () => {
  log("waiting for an available transcontainer..");
}
