package did

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tendermint/abci/example/code"
	"github.com/tendermint/abci/types"
	dbm "github.com/tendermint/tmlibs/db"
)

var (
	stateKey        = []byte("stateKey")
	kvPairPrefixKey = []byte("kvPairKey:")
)

type State struct {
	db      dbm.DB
	Size    int64  `json:"size"`
	Height  int64  `json:"height"`
	AppHash []byte `json:"app_hash"`
}

// TO DO save state as DB file
func loadState(db dbm.DB) State {
	stateBytes := db.Get(stateKey)
	var state State
	if len(stateBytes) != 0 {
		err := json.Unmarshal(stateBytes, &state)
		if err != nil {
			panic(err)
		}
	}
	state.db = db
	return state
}

func saveState(state State) {
	stateBytes, err := json.Marshal(state)
	if err != nil {
		panic(err)
	}
	state.db.Set(stateKey, stateBytes)
}

func prefixKey(key []byte) []byte {
	return append(kvPairPrefixKey, key...)
}

//---------------------------------------------------

var _ types.Application = (*DIDApplication)(nil)

type DIDApplication struct {
	types.BaseApplication

	state State
}

func NewDIDApplication() *DIDApplication {
	state := loadState(dbm.NewMemDB())
	return &DIDApplication{state: state}
}

func (app *DIDApplication) Info(req types.RequestInfo) (resInfo types.ResponseInfo) {
	return types.ResponseInfo{Data: fmt.Sprintf("{\"size\":%v}", app.state.Size)}
}

// ---- Data Structure ----
type Sid struct {
	Namespace string `json:"namespace"`
	Id        string `json:"id"`
}

type MsgDestination struct {
	Users []Sid  `json:"users"`
	Ip    string `json:"ip"`
	Port  string `json:"port"`
}

type Address struct {
	Ip   string `json:"ip"`
	Port string `json:"port"`
}

type CreateRequestParam struct {
	RequestId   string `json:"requestId"`
	MinIdp      int    `json:"minIdp"`
	MessageHash string `json:"messageHash"`
}

type RequestResponse struct {
	Status    string `json:"status"`
	Signature string `json:"signature"`
}

type Request struct {
	RequestId   string            `json:"requestId"`
	MinIdp      int               `json:"minIdp"`
	MessageHash string            `json:"messageHash"`
	Responses   []RequestResponse `json:"response"`
}

type GetRequestParam struct {
	RequestId string `json:"requestId"`
}

type GetRequestResponse struct {
	Status      string `json:"status"`
	MessageHash string `json:"messageHash"`
}

// ---- Data Structure ----

func (app *DIDApplication) DeliverTx(tx []byte) types.ResponseDeliverTx {
	fmt.Println("DeliverTx")
	var key, value []byte

	txString, err := base64.StdEncoding.DecodeString(string(tx))
	if err != nil {
		fmt.Println("error:", err)
		// Handle error can't decode
	}
	fmt.Println(string(txString))
	parts := strings.Split(string(txString), "|")

	method := parts[0]
	param := parts[1]

	if method == "RegisterMsgDestination" {
		fmt.Println("RegisterMsgDestination")
		var msgDestination MsgDestination
		err := json.Unmarshal([]byte(param), &msgDestination)
		if err != nil {
			fmt.Println("error:", err)
			// Handle error can't unmarshal
		}

		for _, user := range msgDestination.Users {
			key := "MsgDestination" + "|" + user.Namespace + "|" + user.Id

			chkExists := app.state.db.Get(prefixKey([]byte(key)))
			if chkExists != nil {

				var addresss []Address
				err := json.Unmarshal([]byte(chkExists), &addresss)
				if err != nil {
					fmt.Println("error:", err)
					// Handle error can't unmarshal
				}

				newAddress := Address{msgDestination.Ip, msgDestination.Port}
				// Check duplicate before add
				chkDup := false
				for _, address := range addresss {
					if newAddress == address {
						chkDup = true
						break
					}
				}

				if chkDup == false {
					addresss = append(addresss, newAddress)
					value, err := json.Marshal(addresss)
					if err != nil {
						fmt.Println("error:", err)
						// Handle error can't marshal
					}
					app.state.db.Set(prefixKey([]byte(key)), []byte(value))
				}

			} else {
				var addresss []Address
				newAddress := Address{msgDestination.Ip, msgDestination.Port}
				addresss = append(addresss, newAddress)
				value, err := json.Marshal(addresss)
				if err != nil {
					fmt.Println("error:", err)
					// Handle error can't marshal
				}
				app.state.db.Set(prefixKey([]byte(key)), []byte(value))
			}
		}

		app.state.Size += 1
		return types.ResponseDeliverTx{
			Code: code.CodeTypeOK,
			Log:  fmt.Sprintf("success")}

	} else if method == "CreateRequest" {
		fmt.Println("CreateRequest")

		var request CreateRequestParam
		err := json.Unmarshal([]byte(param), &request)
		if err != nil {
			fmt.Println("error:", err)
			// Handle error can't unmarshal
		}

		var emptyResponse []RequestResponse
		requestData := Request{request.RequestId, request.MinIdp, request.MessageHash, emptyResponse}

		key := "Request" + "|" + request.RequestId
		value, err := json.Marshal(requestData)
		if err != nil {
			fmt.Println("error:", err)
			// Handle error can't marshal
		}
		app.state.db.Set(prefixKey([]byte(key)), []byte(value))

		return types.ResponseDeliverTx{
			Code: code.CodeTypeOK,
			Log:  fmt.Sprintf("success")}
	} else if method == "CreateIdpResponse" {
		fmt.Println("CreateIDPResponse")
		// TODO add logic for store idp response
		return types.ResponseDeliverTx{
			Code: code.CodeTypeOK,
			Log:  fmt.Sprintf("success")}
	} else {
		fmt.Println("else")
		key, value = tx, tx
		app.state.db.Set(key, value)

		return types.ResponseDeliverTx{
			Code: code.CodeTypeEncodingError,
			Log:  fmt.Sprintf("fail")}
	}
}

func (app *DIDApplication) CheckTx(tx []byte) types.ResponseCheckTx {
	fmt.Println("CheckTx")
	return types.ResponseCheckTx{Code: code.CodeTypeOK}
}

func (app *DIDApplication) Commit() types.ResponseCommit {
	fmt.Println("Commit")
	// Using a memdb - just return the big endian size of the db
	appHash := make([]byte, 8)
	binary.PutVarint(appHash, app.state.Size)
	app.state.AppHash = appHash
	app.state.Height += 1
	saveState(app.state)
	return types.ResponseCommit{Data: appHash}
}

func (app *DIDApplication) Query(reqQuery types.RequestQuery) (resQuery types.ResponseQuery) {
	fmt.Println("Query")
	fmt.Println(string(reqQuery.Data))

	txString, err := base64.StdEncoding.DecodeString(string(reqQuery.Data))
	if err != nil {
		fmt.Println("error:", err)
		// Handle error can't decode
	}
	fmt.Println(string(txString))
	parts := strings.Split(string(txString), "|")

	method := parts[0]
	param := parts[1]

	if method == "GetMsgDestination" {
		fmt.Println("GetMsgDestination")
		var sid Sid
		err := json.Unmarshal([]byte(param), &sid)
		if err != nil {
			fmt.Println("error:", err)
			// Handle error can't unmarshal
		}

		key := "MsgDestination" + "|" + sid.Namespace + "|" + sid.Id
		value := app.state.db.Get(prefixKey([]byte(key)))

		fmt.Println(string(value))
		resQuery.Value = value

		return
	} else if method == "GetRequest" {
		fmt.Println("GetRequest")
		var getRequestParam GetRequestParam
		err := json.Unmarshal([]byte(param), &getRequestParam)
		if err != nil {
			fmt.Println("error:", err)
			// Handle error can't unmarshal
		}

		key := "Request" + "|" + getRequestParam.RequestId
		value := app.state.db.Get(prefixKey([]byte(key)))

		var request Request
		err = json.Unmarshal(value, &request)
		if err != nil {
			fmt.Println("error:", err)
			// Handle error can't unmarshal
		}

		status := "pending"
		acceptCount := 0
		for _, response := range request.Responses {
			if response.Status == "accept" {
				acceptCount++
			} else if response.Status == "reject" {
				status = "reject"
				break
			}
		}

		if acceptCount >= request.MinIdp {
			status = "complete"
		}

		var res GetRequestResponse
		res.Status = status
		res.MessageHash = request.MessageHash

		returnValue, err := json.Marshal(res)
		if err != nil {
			fmt.Println("error:", err)
			// Handle error can't marshal
		}

		fmt.Println(string(returnValue))
		resQuery.Value = returnValue
	} else {
		resQuery.Log = "wrong method name"
		return
	}

	return
}