package proxy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"sync"

	G "gopkg.in/gilmour-libs/gilmour-e-go.v4"
	"gopkg.in/gilmour-libs/gilmour-e-go.v4/backends"
	"time"
)

type Status int

// implements NodeMapOperations
type nodeMap struct {
	sync.Mutex
	regNodes map[NodeID]*Node
}

func initNodeMap() (nm *nodeMap) {
	nm = new(nodeMap)
	nm.regNodes = make(map[NodeID]*Node)
	return
}

var nMap *nodeMap

func InitNodeMap() {
	if nMap != nil {
		return
	}

	nMap = new(nodeMap)
	nMap.regNodes = make(map[NodeID]*Node)

	log.Println("Node Map Initialized")
}

// NodeMapOperations is a interface to enable operation on nodeMap
type NodeMapOperations interface {
	Put(NodeID, *Node) error
	Del(NodeID) error
	Get(NodeID) (*Node, error)
}

// GetNodeMap returns nodeMap
func GetNodeMap() nodeMap {
	return *nMap
}

// Put adds node in nodeMap
func (n *nodeMap) Put(uid NodeID, node *Node) (err error) {
	n.Mutex.Lock()
	defer n.Mutex.Unlock()
	n.regNodes[uid] = node

	return
}

// Del removes node from nodeMap
func (n *nodeMap) Del(uid NodeID) (err error) {
	n.Mutex.Lock()
	defer n.Mutex.Unlock()
	delete(n.regNodes, uid)

	return
}

// Get returns node from nodeMap
func (n *nodeMap) Get(uid NodeID) (node *Node, err error) {
	n.Mutex.Lock()
	defer n.Mutex.Unlock()
	node = n.regNodes[uid]

	if node == nil {
		err = errors.New("Node not found")
	}

	return
}

// Request is a struct for managing requests coming from node
type Request struct {
	Topic       string      `json:"topic"`
	Composition interface{} `json:"composition"`
	Message     interface{} `json:"message"`
	Timeout     int         `json:"timeout"`
}

// RequestResponse is a struct for responding to a Request
type RequestResponse struct {
	Messages interface{} `json:"messages"`
	Code     int         `json:"code"`
	Length   int         `json:"length"`
}

type RequestResponseMessage struct {
	Data interface{} `json:"data"`
	Code int         `json:"code"`
}

// Message is a struct which has data to be processed and handler path for node
type Message struct {
	Data        interface{} `json:"data"`
	HandlerPath string      `json:"handler_path"`
}

type GilmourTopic string

// NodeID is a string to hold node's id
type NodeID string

// ServiceMap is a type of GilmourTopic to string
type ServiceMap map[GilmourTopic]Service

//Node structure
type NodeReq struct {
	Port            string     `json:"port"`
	HealthCheckPath string     `json:"health_check"`
	Slots           []Slot     `json:"slots"`
	Services        ServiceMap `json:"services"`
}

type Node struct {
	port            string
	healthcheckpath string
	slots           []Slot
	services        ServiceMap
	status          Status
	engine          *G.Gilmour
	id              NodeID
}

// Service is a struct which holds details for the service to be added / removed
type Service struct {
	Group        string          `json:"group"`
	Path         string          `json:"path"`
	Timeout      int             `json:"timeout"`
	Data         interface{}     `json:"data"`
	Subscription *G.Subscription `json:"subscription"`
}

// Slot is a struct which holds details for the slot to be added / removed
type Slot struct {
	Topic        string          `json:"topic"`
	Group        string          `json:"group"`
	Path         string          `json:"path"`
	Timeout      int             `json:"timeout"`
	Data         interface{}     `json:"data"`
	Subscription *G.Subscription `json:"subscription"`
}

// Providing NodeOperations
type NodeOperations interface {
	FormatResponse() CreateNodeResponse
	GetPort() string
	GetHealthCheckPath() string
	GetID() string
	GetEngine() *G.Gilmour
	GetNodeDetails(id string) (NodeReq, error)
	GetStatus(sync bool) (int, error)
	GetServices() (ServiceMap, error)

	AddService(GilmourTopic, Service) error
	AddServices(ServiceMap) (err error)
	RemoveService(GilmourTopic, Service) error
	RemoveServices(ServiceMap) error

	RequestService(Request) RequestResponse
	Start() error
	Stop() error
}

//**************************************************************************

// GetPort returns port on which node is running
func (node *Node) GetPort() string {
	return node.port
}

// GetHealthCheckPath returns path on which health check is done
func (node *Node) GetHealthCheckPath() string {
	return node.healthcheckpath
}

// GetID returns node's ID
func (node *Node) GetID() string {
	return string(node.id)
}

// GetEngine returns gilmour backend which gilmour proxy will use
func (node *Node) GetEngine() *G.Gilmour {
	return node.engine
}

// GetServices returns all the services which node is currently subscribed to
func (node *Node) GetServices() (services ServiceMap, err error) {
	if node.status == 200 {
		services = node.services
	}
	return
}

// Stop Exit routine. UnSubscribes Slots, removes registered health ident and triggers backend Stop
func (node *Node) Stop() (err error) {
	node.engine.Stop()
	return
}

func DeleteNode(node *Node) error {
	if _, err := http.Get("http://127.0.0.1:" + node.port); err != nil {
		log.Println(err)
		return err
	}

	if err := nMap.Del(node.id); err != nil {
		log.Println(err)
		return err
	}

	if err := node.Stop(); err != nil {
		log.Println(err)
		return err
	}

	return nil
}

//******************************************************************************
//CreateNodeResponse //init PublishPort

type CreateNodeResponse struct {
	ID          string `json:"id"`
	PublishPort string `json:"publish_port"`
	Status      int    `json:"status"`
}

///////////////////////////////////////////////////////////////////////
func uniqueNodeID(strlen int) (id string) {
	rand.Seed(time.Now().UTC().UnixNano())
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, strlen)
	for i := 0; i < strlen; i++ {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return string(result)
}

// FormatResponse ..... after creating node

func (node *Node) FormatResponse() (resp CreateNodeResponse) {
	resp.ID = string(node.id)
	resp.PublishPort = node.port
	resp.Status, _ = fmt.Printf("%v", node.status)
	return
}

/////////////////////////////////////////////////////////////////////////////////////
// Bind the function with the services

func (service Service) bindListeners(listenPort string, healthPath string) func(req *G.Request, resp *G.Message) {
	return func(req *G.Request, resp *G.Message) {
		message := new(Message)
		if err := req.Data(message); err != nil {
			log.Println(err.Error())
			return
		}
		mJSON, err := json.Marshal(message)
		fmt.Println("Received : ", message, string(mJSON))
		if err != nil {
			log.Println(err.Error())
			return
		}
		requester := fmt.Sprintf("http://localhost:%s/%s", listenPort, service.Path)
		log.Println(requester)
		hndlrResp, err := http.Post(requester, "application/json", bytes.NewBuffer(mJSON))
		body, err := ioutil.ReadAll(hndlrResp.Body)
		if err != nil {
			log.Println(err)
			return
		}
		var data interface{}
		log.Println("Request: ", requester, "Response", hndlrResp)
		err = json.Unmarshal(body, &data)
		if err != nil {
			log.Println("Json Error: ", err.Error())
			return
		}
		resp.SetData(data)
	}
}

//Bind the function with the slots
func (slot Slot) bindListeners(listenPort string, healthPath string) func(req *G.Request) {
	return func(req *G.Request) {
		message := new(Message)
		if err := req.Data(message); err != nil {
			log.Println(err.Error())
			return
		}
		fmt.Println("Received: ", message.Data)
		mJSON, err := json.Marshal(message)
		if err != nil {
			log.Println(err.Error())
			return
		}
		requester := fmt.Sprintf("http://localhost:%s/%s", listenPort, slot.Path)
		log.Println(requester)
		hndlrResp, err := http.Post(requester, "application/json", bytes.NewBuffer(mJSON))
		body, err := ioutil.ReadAll(hndlrResp.Body)
		if err != nil {
			log.Println(err)
			return
		}
		var data interface{}
		log.Println("Request: ", requester, "Response", hndlrResp)
		err = json.Unmarshal(body, &data)
		if err != nil {
			log.Println("Json Error: ", err.Error())
			return
		}
	}
}

// GetSlots returns all the slots on which node is currently subscribed to
func (node *Node) GetSlots() (slots []Slot, err error) {
	if node.status == 200 {
		slots = node.slots
	}
	return
}

// AddService adds and subscribes a service in the existing list of services
func (node *Node) AddService(topic GilmourTopic, service Service) (err error) {
	o := G.NewHandlerOpts()
	o.SetTimeout(service.Timeout)
	o.SetGroup(service.Group)
	if service.Subscription, err = node.engine.ReplyTo(string(topic), service.bindListeners(node.port, node.healthcheckpath), o); err != nil {
		return
	}
	node.services[topic] = service
	return
}

// AddServices adds multiple service's to the existing list of service's by subscribe them
func (node *Node) AddServices(services ServiceMap) (err error) {
	for topic, service := range services {
		if err = node.AddService(topic, service); err != nil {
			log.Println(err)
			return
		}
	}
	return
}

func contains(slots []Slot, slotToAdd Slot) (bool, int) {
	for pos, slot := range slots {
		if slot.Topic == slotToAdd.Topic &&
			slot.Path == slotToAdd.Path &&
			slot.Group == slotToAdd.Group {
			return true, pos
		}
	}
	return false, -1
}

func (node *Node) RemoveService(topic GilmourTopic, service Service) (err error) {
	node.engine.UnsubscribeReply(string(topic), service.Subscription)
	delete(node.services, topic)
	return
}

// AddSlot adds and subscribes a slot in the existing list of slots
func (node *Node) AddSlot(slot Slot) (err error) {
	o := G.NewHandlerOpts()
	o.SetTimeout(slot.Timeout)
	o.SetGroup(slot.Group)
	if slot.Subscription, err = node.engine.Slot(slot.Topic, slot.bindListeners(node.port, node.healthcheckpath), o); err != nil {
		return
	}
	slotExists, pos := contains(node.slots, slot)
	if !slotExists {
		node.slots = append(node.slots, slot)
	} else {
		node.slots[pos].Subscription = slot.Subscription
	}
	return
}

// AddSlots adds multiple slots to the existing list of slot's by subscribe them
func (node *Node) AddSlots(slots []Slot) (err error) {
	for _, slot := range slots {
		if err = node.AddSlot(slot); err != nil {
			log.Println(err.Error())
			return
		}
	}
	return
}

func posByTopic(slots []Slot, topic string) int {
	for p, v := range slots {
		if v.Topic == topic {
			return p
		}
	}
	return -1
}

func posByTopicPath(slots []Slot, topic string, path string) int {
	for p, v := range slots {
		if v.Topic == topic && v.Path == "/"+path {
			return p
		}
	}
	return -1
}

// RemoveSlot removes a slot from the list of slots which node is currently subscribed to
func (node *Node) RemoveSlot(slot Slot) (err error) {
	if slot.Path != "" {
		i := posByTopicPath(node.slots, slot.Topic, slot.Path)
		if i != -1 {
			slotRemove := node.slots[i]
			node.engine.UnsubscribeSlot(slotRemove.Topic, slotRemove.Subscription)
			node.slots = append(node.slots[:i], node.slots[i+1:]...)
		}
	} else {
		for i := posByTopic(node.slots, slot.Topic); i != -1; i = posByTopic(node.slots, slot.Topic) {
			slotRemove := node.slots[i]
			node.engine.UnsubscribeSlot(slotRemove.Topic, slotRemove.Subscription)
			node.slots = append(node.slots[:i], node.slots[i+1:]...)
		}

	}
	return
}

// Start will Start Gilmour engine and added services of slots if any in the Node struct instance
func (node *Node) Start() error {
	if node == nil {
		log.Println("NODE is nill")
	}
	if node.engine == nil {
		return errors.New("Please setup backend engine")
	}
	node.engine.Start()
	if err := node.AddServices(node.services); err != nil {
		return err
	}
	if err := node.AddSlots(node.slots); err != nil {
		return err
	}
	return nil
}

//Getting status of Node and running it

func (node *Node) GetStatus(sync bool) (Status, error) {

	addr := fmt.Sprintf("http://127.0.0.1:%s", node.port)
	log.Println(addr)
	resp, err := http.Get(addr)
	//log.Println(res.Body)
	if err != nil {
		log.Println(err)
		node.status = 500 //dirty
		return node.status, err
	}

	hlp := "/health_check"
	addr = addr + hlp
	req, err := http.Get(addr)

	if err != nil {
		node.status = Status(req.StatusCode) //unavailable
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Println(err)
		return 0, err
	}

	reply := string(body)

	// TODO: validate these assumptions
	if reply == "OK" {
		node.status = 200 //available //ok
	} else {
		node.status = 403 //Unavailable
	}

	return node.status, err

}

// NodeWatchdog checks for a status of node and depending on the status
// If dirty - calls DeleteNode
// If unavailable - calls Stop
// If ok - does nothing
// This exits when node is dirty
func NodeWatchdog(node *Node) {
	stopped := false
	for {
		<-time.After(time.Second * 10)

		status, err := node.GetStatus(true)
		if err != nil {
			log.Println(err.Error())
			// Notify to stakeholders
			continue
		}

		if status == 403 && !stopped {
			stopped = true
			if err = node.Stop(); err != nil {
				log.Println(err.Error())
				return
			}
		} else if (status == 200) && stopped {
			stopped = false
			if err = node.Start(); err != nil {
				log.Println(err.Error())
				return
			}
		} else if status == 404 {
			if err = DeleteNode(node); err != nil {
				log.Println(err.Error())
				return
			}
			return
		}
		node.status = status
	}
}

func MakeGilmour(connect string) (engine *G.Gilmour, err error) {
	redis := backends.MakeRedis(connect, "")
	engine = G.Get(redis)
	return
}

func (node *Node) RequestService(serviceRequest Request) RequestResponse {
	// log.Println("func RequestService serviceRequest Structure: ", serviceRequest)
	message := Message{}
	message.Data = serviceRequest.Message
	message.HandlerPath = node.port
	//Handler Path to be set
	req := node.engine.NewRequest(serviceRequest.Topic)
	resp, err := req.Execute(G.NewMessage().SetData(message))
	if err != nil {
		log.Println("Error running service request: ", err)
		return RequestResponse{}
	}
	output := RequestResponse{}
	if err := resp.Next().GetData(&output); err != nil {
		log.Println("Error receiving response: ", err)
	}
	log.Println("Resp message: ", output)
	return output
}

//Get node details returns the details of the said node id
func GetNodeDetails(id string) (NodeReq, error) {
	nm := GetNodeMap()
	node, err := nm.Get(NodeID(id))
	rep := NodeReq{}
	if err != nil {
		return rep, err
	}
	rep.Port = node.port
	rep.HealthCheckPath = node.healthcheckpath
	rep.Services = node.services
	rep.Slots = node.slots
	return rep, nil
}

func CreateNode(nodeReq *NodeReq, engine *G.Gilmour) (*Node, error) {
	node := new(Node)
	node.engine = engine
	node.id = NodeID(uniqueNodeID(50))
	node.healthcheckpath = nodeReq.HealthCheckPath
	node.port = nodeReq.Port
	node.services = make(ServiceMap)
	node.services = nodeReq.Services
	node.slots = nodeReq.Slots
	if node.engine == nil {
		log.Println("Engine is nil")
	}

	var err error
	node.status, err = node.GetStatus(true)
	if err != nil {
		log.Printf("Cannot get status of a node: %+v", err)
		return nil, err
	}

	err = nMap.Put(node.id, node)
	return node, nil
}
