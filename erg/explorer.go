package erg

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"strconv"

	"github.com/hashicorp/go-retryablehttp"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var (
	oracleAddress = "4FC5xSYb7zfRdUhm6oRmE11P2GJqSMY8UARPbHkmXEq6hTinXq4XNWdJs73BEV44MdmJ49Qo"
)

type Explorer struct {
	client *retryablehttp.Client
	url *url.URL
	user string
	pass string

	logger *log.Entry
}

func NewExplorer(client *retryablehttp.Client, logger *log.Entry) (*Explorer, error) {
	var node *Explorer
	var u *url.URL

	u = &url.URL{
		Scheme: viper.Get("explorer_node.scheme").(string),
		Host: viper.Get("explorer_node.fqdn").(string)+":"+strconv.Itoa(viper.Get("explorer_node.port").(int)),
	}

	node = &Explorer{
		client:     client,
		logger:     logger,
		url:        u,
	    user:       viper.Get("ergo_node.user").(string),
	    pass:       viper.Get("ergo_node.password").(string),
	}

	return node, nil
}

func (e *Explorer) GetOracleTxs(minHeight, maxHeight int) (ErgBoxIds, error) {
	var ergTxs ErgBoxIds

	endpoint := fmt.Sprintf("%s/api/v1/addresses/%s/transactions?fromHeight=%d&toHeight=%d", e.url.String(), oracleAddress, minHeight, maxHeight)
	req, err := retryablehttp.NewRequest("GET", endpoint, nil)
	if err != nil {
		return ergTxs, fmt.Errorf("failed to build oracle transactions request - %s", err.Error())
	}
	req.Header.Set("no-backend-func", "GetOracleTxs")

	resp, err := e.client.Do(req)
	if err != nil {
		return ergTxs, fmt.Errorf("error calling ergo api explorer - %s", err.Error())
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ergTxs, fmt.Errorf("error reading erg txs body - %s", err.Error())
	}

	err = json.Unmarshal(body, &ergTxs)
	if err != nil {
		return ergTxs, fmt.Errorf("error unmarshalling erg txs - %s", err.Error())
	}

	return ergTxs, nil
}