package signallingclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/pion/webrtc/v3"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"time"
)

const GET_OFFER_RETRY_INTERVAL = 5
const GET_ANSWER_RETRY_INTERVAL = 1

type SignallingClient struct{
	Url string
}

func NewSignallingClient(url string) *SignallingClient {
	return &SignallingClient{Url: url}
}

func (signalingClient *SignallingClient) GetQueue() (string, error){

	url := signalingClient.Url + "/signaling/1.0/client/queue"
	response, err := http.Get(url)
	if err != nil {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"error": err.Error(),
			}).Error("failed to get a free connection, will retry in ", GET_OFFER_RETRY_INTERVAL, "s.")
		time.Sleep(GET_OFFER_RETRY_INTERVAL * time.Second)
		return signalingClient.GetQueue()
	}

	body, err := ioutil.ReadAll(response.Body)

	if response.StatusCode != http.StatusOK {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"httpCode": response.StatusCode,
				"error": string(body),
			}).Warn("no waiting offers are available. will retry in ", GET_OFFER_RETRY_INTERVAL, "s.")
		time.Sleep(GET_OFFER_RETRY_INTERVAL * time.Second)
		return signalingClient.GetQueue()
	}

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
		}).Info("got a valid client connectionId")

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

	body, err := ioutil.ReadAll(response.Body)

	if response.StatusCode != http.StatusOK {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"httpCode": response.StatusCode,
				"connectionId": connectionId,
				"error": string(body),
			}).Error("failed to get a valid response for getOffer")
		return nil,  errors.New("failed to get connection")
	}

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
		}).Info("got a valid client offer")

	return &offer, nil
}

func (signalingClient *SignallingClient) SendOffer(offer webrtc.SessionDescription, connectionId string) (string, error) {

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

	url := signalingClient.Url + "/signaling/1.0/application/connections?connectionId=" + connectionId
	response, err := http.Post(url, "application/json", bytes.NewBuffer(requestBody))

	// error sending e.g. timeout
	if err != nil {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"error": err.Error(),
			}).Error("failed to post an offer")
		return "", err
	}

	// post succeeded, read body
	// we read the body even if status code is not success in order to read the error
	body, err := ioutil.ReadAll(response.Body)

	// error reading response body
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

	if response.StatusCode != http.StatusCreated {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"httpCode": response.StatusCode,
				"error": string(body),
			}).Error("failed to post an offer")
		return "", errors.New("failed to send offer")
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
		}).Info("got a valid application connectionId")

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

	// post succeeded, read body
	// we read the body even if status code is not success in order to read the error
	body, err := ioutil.ReadAll(response.Body)

	if response.StatusCode != http.StatusCreated {
		log.WithFields(
			log.Fields{
				"component": "signallingClient",
				"url": url,
				"httpCode": response.StatusCode,
				"error": string(body),
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

func (signalingClient *SignallingClient) GetAnswer(connectionId string, tries int, callback func(*webrtc.SessionDescription, error)) (context.Context, context.CancelFunc) {

	ctx, ctxCancel := context.WithCancel(context.Background())
	go signalingClient.getAnswer(connectionId, tries, 0, callback, ctx, ctxCancel)
	return ctx, ctxCancel
}

func (signalingClient *SignallingClient) getAnswer(connectionId string, tries int, currentTry int, callback func(*webrtc.SessionDescription, error), ctx context.Context, ctxCancel context.CancelFunc) {

	if ctx.Err() != nil {
		callback(nil, nil)
		return
	}

	url := signalingClient.Url + "/signaling/1.0/connections/" + connectionId + "/answer"
	response, err := http.Get(url)
	if err != nil {
		log.WithFields(
			log.Fields{
				"component":    "signallingClient",
				"url":          url,
				"connectionId": connectionId,
				"error":        err,
			}).Warnf("try #%d/%d: no server answer available yet. request failed.", currentTry, tries)
		time.Sleep(GET_ANSWER_RETRY_INTERVAL * time.Second)
		currentTry++
		if currentTry == tries {
			log.WithFields(
				log.Fields{
					"component":    "signallingClient",
					"url":          url,
					"connectionId": connectionId,
					"error":        err,
				}).Error("failed to get an answer for connectionId. giving up..")
			callback(nil, errors.New("failed to get answer after retires"))
			return
		}
		signalingClient.getAnswer(connectionId, tries, currentTry, callback, ctx, ctxCancel)
		return
	}

	// we read the body even if status code is not success in order to read the error
	body, err := ioutil.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK {
		log.WithFields(
			log.Fields{
				"component":    "signallingClient",
				"url":          url,
				"httpCode":     response.StatusCode,
				"connectionId": connectionId,
				"error":        string(body),
			}).Warnf("try #%d/%d: no server answer available yet", currentTry, tries)
		time.Sleep(GET_ANSWER_RETRY_INTERVAL * time.Second)
		currentTry++
		if currentTry == tries {
			log.WithFields(
				log.Fields{
					"component":    "signallingClient",
					"url":          url,
					"connectionId": connectionId,
				}).Error("failed to get an answer for connectionId. giving up..")
			callback(nil, errors.New("failed to get answer after retires"))
			return
		}
		signalingClient.getAnswer(connectionId, tries, currentTry, callback, ctx, ctxCancel)
		return
	}

	if err != nil {
		log.WithFields(
			log.Fields{
				"component":    "signallingClient",
				"url":          url,
				"httpCode":     response.StatusCode,
				"connectionId": connectionId,
				"error":        err.Error(),
			}).Error("failed to read answer response")
		callback(nil, err)
		return
	}

	var answer webrtc.SessionDescription
	err = json.Unmarshal(body, &answer)
	if err != nil {
		log.WithFields(
			log.Fields{
				"component":    "signallingClient",
				"url":          url,
				"httpCode":     response.StatusCode,
				"connectionId": connectionId,
				"error":        err.Error(),
			}).Error("failed to convert response to a valid webrtc answer")
		callback(nil, err)
		return
	}

	log.WithFields(
		log.Fields{
			"component":    "signallingClient",
			"url":          url,
			"httpCode":     response.StatusCode,
			"connectionId": connectionId,
		}).Info("got a valid application answer")

	ctx.Done()
	callback(&answer, nil)
	return
}
