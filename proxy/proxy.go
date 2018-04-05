package proxy

import (
	"log"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"io/ioutil"
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


// ReqOpts is a struct for setting options like timeout while making a request
type ReqOpts struct {
	Timeout int `json:"timeout"`
}

// Request is a struct for managing requests coming from node
type Request struct {
	Topic       string      `json:"topic"`
	Composition interface{} `json:"composition"`
	Message     interface{} `json:"message"`
	Opts        ReqOpts     `json:"opts"`
}

// RequestResponse is a struct for responding to a Request
type RequestResponse struct {
	Messages map[string]interface{} `json:"messages"`
	Code     int                    `json:"code"`
	Length   int                    `json:"length"`
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
	Services        ServiceMap `json:"services"`
}

type Node struct {
	port 			string
	healthcheckpath string
	services 		ServiceMap
	status			Status
	engine			*G.Gilmour
	id				NodeID
}

// Service is a struct which holds details for the service to be added / removed
type Service struct {
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
	GetStatus(sync bool) (int, error)
	GetServices() (ServiceMap, error)

	AddService(GilmourTopic, Service) error
	AddServices(ServiceMap) (err error)
	RemoveService(GilmourTopic, Service) error
	RemoveServices(ServiceMap) error

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

func DeleteNode(node *Node) (error){
	if _, err := http.Get("http://127.0.0.1:" + node.port); err != nil {
		log.Println(err)
		return err
	}

	if err := nMap.Del(node.id); err!= nil {
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
	ID            string `json:"id"`
	PublishPort   string `json:"publish_port"`
	Status        int `json:"status"`
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
	resp.Status,_ = fmt.Printf("%v", node.status)
	return
}


/////////////////////////////////////////////////////////////////////////////////////
// Bind the function with the services

func (service Service) bindListeners(listenPort string, healthPath string) func(req *G.Request, resp *G.Message) {
	return func(req *G.Request, resp *G.Message){
		message := new(Message)
		if err := req.Data(message); err!= nil{
			log.Println(err.Error())
			return
		}
		fmt.Println("Received : ", message.Data)
		mJSON, err := json.Marshal(message)
		if err != nil {
			log.Println(err.Error())
			return
		}
		hndlrResp , err := http.Post("http://127.0.0.1:"+listenPort,"application/json",bytes.NewReader(mJSON))
		body, err := ioutil.ReadAll(hndlrResp.Body)
		if err != nil {
			log.Println(err)
			panic(err)
		}
		var data interface{}
		err = json.Unmarshal(body, &data)
		if err != nil {
			log.Println("Error: ", err.Error())
			return
		}
		resp.SetData(data)

	}
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


// Start will Start Gilmour engine and added services of slots if any in the Node struct instance
func (node *Node) Start() (error) {
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
	return nil
}

//Getting status of Node and running it

func (node *Node) GetStatus(sync bool)(Status, error){

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

	body ,err := ioutil.ReadAll(req.Body)
	if err != nil{
		log.Println(err)
		return 0, err
	}

	reply := string(body)

	// TODO: validate these assumptions
	if reply == "OK"{
		node.status = 200 //available //ok
	} else {
		node.status = 403 //Unavailable
	}

	return node.status,err
	
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

func CreateNode(nodeReq *NodeReq, engine *G.Gilmour) (*Node, error){
	node := new(Node)
	node.engine = engine
	node.id = NodeID(uniqueNodeID(50))
	node.healthcheckpath = nodeReq.HealthCheckPath
	node.port = nodeReq.Port
	node.services = make(ServiceMap)
	node.services = nodeReq.Services

	if node.engine == nil {
		log.Println("Engine is nil")
	}

	var err error
	node.status, err = node.GetStatus(true)
	if err != nil {
		log.Printf("Cannot get status of a node: %+v", err)
		return nil, err
	}

	err = nMap.Put(node.id,node)
	return node, nil
}