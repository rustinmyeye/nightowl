package erg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/nightowlcasino/nightowl/config"
	"github.com/spf13/viper"
)

var (
	walletLock        				= "/wallet/lock"
	walletUnlock      				= "/wallet/unlock"
	postErgTx         				= "/wallet/transaction/send"
	getUtxoBox        				= "/utxo/byId/"
	getLastHeaders    				= "/blocks/lastHeaders/1"
	getUnconfirmedTxs 				= "/transactions/unconfirmed"
	getUnconfirmedOutputsByErgoTree = "/transactions/unconfirmed/outputs/byErgoTree"
	getTxFee          				= "/transactions/getFee"
	ergoTreeToAddr    				= "/utils/ergoTreeToAddress/"
	serializeBox      				= "/utxo/withPool/byIdBinary/"
)

type ErgNode struct {
	client *retryablehttp.Client
	url *url.URL
	user string
	pass string
	apiKey string
	walletPass string
}

func NewErgNode(client *retryablehttp.Client) (*ErgNode, error) {
	var node *ErgNode

	config.SetNodeDefaults()

	var u = &url.URL{
		Scheme: viper.Get("ergo_node.scheme").(string),
		Host: viper.Get("ergo_node.fqdn").(string)+":"+strconv.Itoa(viper.Get("ergo_node.port").(int)),
	}

	node = &ErgNode{
		client:     client,
		url:        u,
	    user:       viper.Get("ergo_node.user").(string),
	    pass:       viper.Get("ergo_node.password").(string),
	    apiKey:     viper.Get("ergo_node.api_key").(string),
	    walletPass: viper.Get("ergo_node.wallet_password").(string),
	}

	return node, nil
}

func (n *ErgNode) unlockWallet() ([]byte, error) {
	var ret []byte

	endpoint := fmt.Sprintf("%s%s", n.url.String(), walletUnlock)
	body := bytes.NewBuffer([]byte(fmt.Sprintf("{\"pass\": \"%s\"}", n.walletPass)))

	req, err := retryablehttp.NewRequest("POST", endpoint, body)
	if err != nil {
		return ret, fmt.Errorf("error creating erg node unlock wallet request - %s", err.Error())
	}
	req.SetBasicAuth(n.user, n.pass)
	req.Header.Set("api_key", n.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return ret, fmt.Errorf("error unlocking erg node wallet - %s", err.Error())
	}

	ret, err = io.ReadAll(resp.Body)
	if err != nil {
		return ret, fmt.Errorf("error parsing erg node unlock response - %s", err.Error())
	}

	return ret, nil
}

func (n *ErgNode) lockWallet() ([]byte, error) {
	var ret []byte

	endpoint := fmt.Sprintf("%s%s", n.url.String(), walletLock)

	req, err := retryablehttp.NewRequest("GET", endpoint, nil)
	if err != nil {
		return ret, fmt.Errorf("error creating erg node lock wallet request - %s", err.Error())
	}
	req.SetBasicAuth(n.user, n.pass)
	req.Header.Set("api_key", n.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return ret, fmt.Errorf("error locking erg node wallet - %s", err.Error())
	}

	ret, err = io.ReadAll(resp.Body)
	if err != nil {
		return ret, fmt.Errorf("error parsing erg node lock response - %s", err.Error())
	}

	return ret, nil
}

func (n *ErgNode) GetCurrenHeight() (int, error) {
	var header ErgHeader
	var height int

	endpoint := fmt.Sprintf("%s%s", n.url.String(), getLastHeaders)

	req, err := retryablehttp.NewRequest("GET", endpoint, nil)
	if err != nil {
		return height, fmt.Errorf("error creating block last headers request - %s", err.Error())
	}
	req.SetBasicAuth(n.user, n.pass)
	req.Header.Set("api_key", n.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return height, fmt.Errorf("error calling block last headers - %s", err.Error())
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return height, fmt.Errorf("error parsing block last headers response - %s", err.Error())
	}

	err = json.Unmarshal(body, &header)
	if err != nil {
		return height, fmt.Errorf("error unmarshalling block last headers response - %s", err.Error())
	}

	height = header[0].Height

	return height, nil
}

func (n *ErgNode) GetUnconfirmedTxs(limit, offset int) ([]ErgTxUnconfirmed, error) {
	var txs []ErgTxUnconfirmed

	endpoint := fmt.Sprintf("%s%s?limit=%d&offset=%d", n.url.String(), getUnconfirmedTxs, limit, offset)

	req, err := retryablehttp.NewRequest("GET", endpoint, nil)
	if err != nil {
		return txs, fmt.Errorf("error creating unconfirmed txs request - %s", err.Error())
	}
	req.SetBasicAuth(n.user, n.pass)
	req.Header.Set("api_key", n.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return txs, fmt.Errorf("error calling GetUnconfirmedTxs - %s", err.Error())
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return txs, fmt.Errorf("error reading erg txs body - %s", err.Error())
	}

	err = json.Unmarshal(body, &txs)
	if err != nil {
		return txs, fmt.Errorf("error unmarshalling unconfirmed Txs body - %s", err.Error())
	}

	return txs, nil
}

func (n *ErgNode) GetUnconfirmedOutputsByErgoTree(ergoTree string, limit, offset int) ([]ErgTxOutputNode, error) {
	var outputs []ErgTxOutputNode

	endpoint := fmt.Sprintf("%s%s?limit=%d&offset=%d", n.url.String(), getUnconfirmedOutputsByErgoTree, limit, offset)

	payload, err := json.Marshal(ergoTree)
	if err != nil {
		return outputs, fmt.Errorf("error marshalling unconfirmed tx outputs payload - %s", err.Error())
	}

	req, err := retryablehttp.NewRequest("POST", endpoint, bytes.NewBuffer(payload))
	if err != nil {
		return outputs, fmt.Errorf("error creating unconfirmed tx outputs request - %s", err.Error())
	}
	req.SetBasicAuth(n.user, n.pass)
	req.Header.Set("api_key", n.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return outputs, fmt.Errorf("error calling GetUnconfirmedOutputsByErgoTree - %s", err.Error())
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return outputs, fmt.Errorf("error reading erg utxo outputs body - %s", err.Error())
	}

	err = json.Unmarshal(body, &outputs)
	if err != nil {
		return outputs, fmt.Errorf("error unmarshalling unconfirmed utxo outputs body - %s", err.Error())
	}

	return outputs, nil
}

func (n *ErgNode) PostErgOracleTx(payload []byte) ([]byte, error) {
	var ret []byte

	_, err := n.unlockWallet()
	if err != nil {
		return ret, err
	}

	defer n.lockWallet()

	endpoint := fmt.Sprintf("%s%s", n.url.String(), postErgTx)
	body := bytes.NewBuffer(payload)

	req, err := retryablehttp.NewRequest("POST", endpoint, body)
	if err != nil {
		return ret, fmt.Errorf("error creating postErgOracleTx request - %s", err.Error())
	}
	req.SetBasicAuth(n.user, n.pass)
	req.Header.Set("api_key", n.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return ret, fmt.Errorf("error submitting erg tx to node - %s", err.Error())
	}

	// Some was wrong, report the error
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return ret, fmt.Errorf("response status code %d", resp.StatusCode)
	}

	ret, err = io.ReadAll(resp.Body)
	if err != nil {
		return ret, fmt.Errorf("error parsing erg tx response - %s", err.Error())
	}

	return ret, nil
}

func (n *ErgNode) SerializeErgBox(boxId string) (string, error) {
	var bytes Serialized

	endpoint := fmt.Sprintf("%s%s%s", n.url.String(), serializeBox, boxId)

	req, err := retryablehttp.NewRequest("GET", endpoint, nil)
	if err != nil {
		return bytes.Bytes, fmt.Errorf("error creating SerializeErgBox request - %s", err.Error())
	}
	req.SetBasicAuth(n.user, n.pass)

	resp, err := n.client.Do(req)
	if err != nil {
		return bytes.Bytes, fmt.Errorf("error getting serializing erg box - %s", err.Error())
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return bytes.Bytes, fmt.Errorf("error parsing serialized erg box response - %s", err.Error())
	}

	err = json.Unmarshal(body, &bytes)
	if err != nil {
		return bytes.Bytes, fmt.Errorf("error unmarshalling serialized erg box response - %s", err.Error())
	}

	return bytes.Bytes, nil
}

func (n *ErgNode) GetErgUtxoBox(boxId string) (ErgTxOutputNode, error) {
	var utxo ErgTxOutputNode

	endpoint := fmt.Sprintf("%s%s%s", n.url.String(), getUtxoBox, boxId)

	req, err := retryablehttp.NewRequest("GET", endpoint, nil)
	if err != nil {
		return utxo, fmt.Errorf("error creating getErgBoxes request - %s", err.Error())
	}
	req.SetBasicAuth(n.user, n.pass)

	resp, err := n.client.Do(req)
	if err != nil {
		return utxo, fmt.Errorf("error getting erg utxo box - %s", err.Error())
	}

	if resp.StatusCode == 404 {
		return utxo, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return utxo, fmt.Errorf("error parsing erg utxo box response - %s", err.Error())
	}

	err = json.Unmarshal(body, &utxo)
	if err != nil {
		return utxo, fmt.Errorf("error unmarshalling erg utxo box response - %s", err.Error())
	}

	return utxo, nil
}

func (n *ErgNode) ErgoTreeToAddress(ergoTree string) (string, error) {
	var address map[string]interface{}

	endpoint := fmt.Sprintf("%s%s%s", n.url.String(), ergoTreeToAddr, ergoTree)

	req, err := retryablehttp.NewRequest("GET", endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("error creating ergoTreeToAddress request - %s", err.Error())
	}
	req.SetBasicAuth(n.user, n.pass)

	resp, err := n.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error getting erg tree address - %s", err.Error())
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error parsing erg tree address response - %s", err.Error())
	}

	err = json.Unmarshal(body, &address)
	if err != nil {
		return "", fmt.Errorf("error unmarshalling erg tree address response - %s", err.Error())
	}

	return address["address"].(string), nil
}

func (n *ErgNode) GetTxFee(txSize int) (int, error) {
	var fee int

	endpoint := fmt.Sprintf("%s%s?waitTime=1&txSize=%d", n.url.String(), getTxFee, txSize)

	req, err := retryablehttp.NewRequest("GET", endpoint, nil)
	if err != nil {
		return fee, fmt.Errorf("error creating getTxFee request - %s", err.Error())
	}
	req.SetBasicAuth(n.user, n.pass)

	resp, err := n.client.Do(req)
	if err != nil {
		return fee, fmt.Errorf("error getting erg tx fee - %s", err.Error())
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fee, fmt.Errorf("error parsing erg tx fee response - %s", err.Error())
	}

	fee, _ = strconv.Atoi(string(body))

	return fee, nil
}