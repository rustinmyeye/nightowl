package rng

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
	log "github.com/sirupsen/logrus"
)

const (
	SLICE_SIZE = 20
)

type Service struct {
	component string
	logger    *log.Entry
	Nats      *nats.Conn
}

type CombinedHashes struct {
	Hash  string   `json:"hash"`
	Boxes []string `json:"boxes"`
}

var (
	index = 0
	combinedHashes = make([]CombinedHashes, SLICE_SIZE)
)

func NewService(logger *log.Entry, nats *nats.Conn) (service *Service, err error) {
	service = &Service{
		component: "rng",
		logger:    logger,
		Nats:      nats,
	}

	if _, err = nats.Subscribe("eth.hash", service.handleNATSMessages); err != nil {
		return nil, err
	}
	logger.Info("subscribed to eth.hash")

	return service, err
}

// handleNATSMessages is called on receipt of a new NATS message.
func (s *Service) handleNATSMessages(msg *nats.Msg) {
	var hash CombinedHashes
	err := json.Unmarshal(msg.Data, &hash)
	if err != nil {
		s.logger.WithFields(log.Fields{"error": err.Error()}).Error("failed to unmarshal CombinedHashes")
	} else {
		combinedHashes[index%SLICE_SIZE] = hash
		// the ERG BoxId random numbers are stored in a hash map and will be set to the 2nd ETH hash block Id
		// from the initially associated one
		if index >= 2 {
			h := combinedHashes[(index-2)%SLICE_SIZE]
			for _, boxId := range h.Boxes {
				allErgBlockRandNums.randNums[boxId] = hash.Hash[2:10]
			}
		}

		// when the slice size has reached 20 we will begin to remove old ERG BoxIds from the hash map
		if index >= 19 {
			old := combinedHashes[(index+1)%SLICE_SIZE]
			for _, boxId := range old.Boxes {
				allErgBlockRandNums.Delete(boxId)
			}
		}

		index++
	}
}