package signallingclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/pion/webrtc/v3"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"time"
)

const RETRY_INTERVAL = 5

type SignallingClient struct{
	Url string
}

func NewSignallingClient(url string) *SignallingClient {
	return &SignallingClient{Url: url}
}

func (signalingClient *SignallingClient) GetQueue() (string, error){

	url := signalingClient.Url + "/signaling/1.0/queue"
	response, err := http.Get(url)
	if err != nil {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"error": err.Error(),
			}).Error("failed to get a free connection, will retry in ", RETRY_INTERVAL, "s.")
		time.Sleep(RETRY_INTERVAL * time.Second)
		return signalingClient.GetQueue()
	}

	if response.StatusCode != http.StatusOK {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"httpCode": response.StatusCode,
			}).Warn("no waiting offers are available. will retry in ", RETRY_INTERVAL, "s.")
		time.Sleep(RETRY_INTERVAL * time.Second)
		return signalingClient.GetQueue()
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"httpCode": response.StatusCode,
				"error": err.Error(),
			}).Error("failed to read getQueue response")
		return "", err
	}

	connection := map[string]string{}
	err = json.Unmarshal(body, &connection)
	if err != nil {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"httpCode": response.StatusCode,
				"error": err.Error(),
			}).Error("failed to parse getQueue response")
		return "", err
	}

	if _, ok := connection["connectionId"]; !ok {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"httpCode": response.StatusCode,
				"error": err.Error(),
			}).Error("failed to get connectionId from connection response")
	}

	log.WithFields(
		log.Fields{
			"component": "signallingClient",
			"url": url,
			"httpCode": response.StatusCode,
			"connectionId": connection["connectionId"],
		}).Info("got a valid connectionId")

	return connection["connectionId"], nil

}

func (signalingClient *SignallingClient) GetOffer(connectionId string) (*webrtc.SessionDescription, error) {

	url := signalingClient.Url + "/signaling/1.0/connections/" + connectionId + "/offer"
	response, err := http.Get(url)
	if err != nil {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"connectionId": connectionId,
				"error": err.Error(),
			}).Error("failed to get an offer for connectionId")
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"httpCode": response.StatusCode,
				"connectionId": connectionId,
				"error": err.Error(),
			}).Error("failed to get a valid response for getOffer")
		return nil,  errors.New("failed to get connection")
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"httpCode": response.StatusCode,
				"connectionId": connectionId,
				"error": err.Error(),
			}).Error("failed to read offer response")
		return nil, err
	}

	var offer webrtc.SessionDescription
	err = json.Unmarshal(body, &offer)
	if err != nil {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"httpCode": response.StatusCode,
				"connectionId": connectionId,
				"error": err.Error(),
			}).Error("failed to convert response to a valid webrtc offer")
		return nil, err
	}

	log.WithFields(
		log.Fields{
			"component": "signallingClient",
			"url": url,
			"httpCode": response.StatusCode,
			"connectionId": connectionId,
		}).Info("got a valid offer")

	return &offer, nil
}

func (signalingClient *SignallingClient) SendOffer(offer webrtc.SessionDescription) (string, error) {

	requestBody, err := json.Marshal(struct{
		Type webrtc.SDPType `json:"type"`
		SDP  string  `json:"sdp"`
		DeviceId    string `json:"deviceId"`
	} {
		Type: offer.Type,
		SDP: offer.SDP,
		DeviceId: "transcontainer",
	})

	if err != nil {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"error": err.Error(),
			}).Error("failed to parse webrtc offer to string")
		return "", err
	}

	url := signalingClient.Url + "/signaling/1.0/connections/"
	response, err := http.Post(url, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"error": err.Error(),
			}).Error("failed to post an offer")
		return "", err
	}

	if response.StatusCode != http.StatusCreated {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"httpCode": response.StatusCode,
			}).Error("failed to post an offer")
		return "", errors.New("failed to send offer")
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"httpCode": response.StatusCode,
				"error": err.Error(),
			}).Error("failed to read sendOffer response")
		return "", err
	}

	connection := map[string]string{}
	err = json.Unmarshal(body, &connection)
	if err != nil {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"httpCode": response.StatusCode,
				"error": err.Error(),
			}).Error("failed to parse sendOffer response")
		return "", err
	}

	if _, ok := connection["connectionId"]; !ok {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"httpCode": response.StatusCode,
				"error": err.Error(),
			}).Error("failed to get connectionId from connection response")
		return "", err
	}

	log.WithFields(
		log.Fields{
			"component": "signallingClient",
			"url": url,
			"httpCode": response.StatusCode,
			"connectionId": connection["connectionId"],
		}).Info("got a valid connectionId")

	return connection["connectionId"], nil
}

func (signalingClient *SignallingClient) SendAnswer(connectionId string, answer webrtc.SessionDescription) error {

	requestBody, err := json.Marshal(answer)
	if err != nil {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"error": err.Error(),
			}).Error("failed to parse webrtc answer to string")
		return err
	}

	url := signalingClient.Url + "/signaling/1.0/connections/" + connectionId + "/answer"
	response, err := http.Post(url, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"error": err.Error(),
			}).Error("failed to post an answer")
		return err
	}

	if response.StatusCode != http.StatusCreated {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"httpCode": response.StatusCode,
			}).Error("failed to post an answer")
		return errors.New("failed to send answer")
	}

	log.WithFields(
		log.Fields{
			"component": "signallingClient",
			"url": url,
			"httpCode": response.StatusCode,
			"connectionId": connectionId,
		}).Info("posted an answer successfully")

	return nil
}

func (signalingClient *SignallingClient) GetAnswer(connectionId string, tries int) (*webrtc.SessionDescription, error) {

	url := signalingClient.Url + "/signaling/1.0/connections/" + connectionId + "/answer"
	response, err := http.Get(url)
	if err != nil {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"connectionId": connectionId,
				"error": err,
			}).Warn("failed to get an answer for connectionId")
		time.Sleep(RETRY_INTERVAL * time.Second)
		tries--
		if tries == 0 {
			log.WithFields(
				log.Fields{
					"component":    "signallingClient",
					"url":          url,
					"connectionId": connectionId,
					"error":        err,
				}).Error("failed to get an answer for connectionId. giving up..")
			return nil, errors.New("failed to get answer after retires")
		}
		return signalingClient.GetAnswer(connectionId, tries)
	}

	if response.StatusCode != http.StatusOK {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"httpCode": response.StatusCode,
				"connectionId": connectionId,
			}).Warn("failed to get a valid response for getAnswer")
		time.Sleep(RETRY_INTERVAL * time.Second)
		tries--
		if tries == 0 {
			log.WithFields(
				log.Fields{
					"component":    "signallingClient",
					"url":          url,
					"connectionId": connectionId,
				}).Error("failed to get an answer for connectionId. giving up..")
			return nil, errors.New("failed to get answer after retires")
		}
		return signalingClient.GetAnswer(connectionId, tries)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"httpCode": response.StatusCode,
				"connectionId": connectionId,
				"error": err.Error(),
			}).Error("failed to read answer response")
		return nil, err
	}

	var answer webrtc.SessionDescription
	err = json.Unmarshal(body, &answer)
	if err != nil {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"httpCode": response.StatusCode,
				"connectionId": connectionId,
				"error": err.Error(),
			}).Error("failed to convert response to a valid webrtc answer")
		return nil, err
	}

	log.WithFields(
		log.Fields{
			"component": "signallingClient",
			"url": url,
			"httpCode": response.StatusCode,
			"connectionId": connectionId,
		}).Info("got a valid answer")

	return &answer, nil
}

func (signalingClient *SignallingClient) GetIce(connectionId string, pc *webrtc.PeerConnection) error {

	url := signalingClient.Url + "/signaling/1.0/connections/" + connectionId + "/ice"
	response, err := http.Get(url)
	if err != nil {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"connectionId": connectionId,
				"error": err.Error(),
			}).Error("failed to get ice for connectionId")
		return err
	}

	if response.StatusCode != http.StatusOK {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"httpCode": response.StatusCode,
			}).Warn("no ice candidates are available. querying again in 5s..")
		time.Sleep(5 * time.Second)
		return signalingClient.GetIce(connectionId, pc)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"httpCode": response.StatusCode,
				"connectionId": connectionId,
				"error": err.Error(),
			}).Error("failed to read ice response")
		return err
	}

	err = pc.AddICECandidate(webrtc.ICECandidateInit{Candidate: string(body)})
	if err != nil {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"httpCode": response.StatusCode,
				"connectionId": connectionId,
				"error": err.Error(),
			}).Error("failed to add ice candidate to peer connection")
		return err
	}

	log.WithFields(
		log.Fields{
			"component": "signallingClient",
			"url": url,
			"httpCode": response.StatusCode,
			"connectionId": connectionId,
		}).Info("added ice candidate to peer connection successfully")

	return nil;
}
