package rng

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	SLICE_SIZE = 20
)

type Service struct {
	component string
	nats      *nats.Conn
}

type CombinedHashes struct {
	Hash  string   `json:"hash"`
	Boxes []string `json:"boxes"`
}

var (
	index = 0
	combinedHashes = make([]CombinedHashes, SLICE_SIZE)
	log *zap.Logger
)

func NewService(nats *nats.Conn) (service *Service, err error) {

	log = zap.L()

	service = &Service{
		component: "rng",
		nats:      nats,
	}

	if _, err = nats.Subscribe(viper.Get("nats.random_number_subj").(string), service.handleNATSMessages); err != nil {
		return nil, err
	}
	log.Info("successfully subscribed to " + viper.Get("nats.random_number_subj").(string))

	return service, err
}

// handleNATSMessages is called on receipt of a new NATS message.
func (s *Service) handleNATSMessages(msg *nats.Msg) {
	var hash CombinedHashes
	err := json.Unmarshal(msg.Data, &hash)
	if err != nil {
		log.Error("failed to unmarshal CombinedHashes", zap.Error(err))
	} else {
		combinedHashes[index%SLICE_SIZE] = hash
		// the ERG BoxId random numbers are stored in a hash map and will be set to the next drand hash number
		// from the initially associated one
		if index >= 1 {
			h := combinedHashes[(index-1)%SLICE_SIZE]
			for _, boxId := range h.Boxes {
				allErgBlockRandNums.randNums[boxId] = hash.Hash[0:8]
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