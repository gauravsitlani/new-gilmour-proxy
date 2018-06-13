//Proxy Backup = Proxy > 5/7/18

package main

import (
	"./proxy"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io/ioutil"
	"log"
	"net/http"
)

func getNode(id string) (node *proxy.Node, err error) {
	nm := proxy.GetNodeMap()
	node, err = nm.Get(proxy.NodeID(id))
	return
}

func formatResponse(key string, value interface{}) interface{} {
	js, err := json.Marshal(map[string]interface{}{key: value})
	if err != nil {
		log.Println(err)
		js = []byte(err.Error())
	}
	return js
}

func setResponseStatus(err error) string {
	status := "ok"
	if err != nil {
		status = err.Error()
	}
	return status
}

func logWriterError(w http.ResponseWriter, err error) {
	errStr := err.Error()
	log.Println(errStr)
	js := formatResponse("error", errStr)
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(js.([]byte)); err != nil {
		log.Println(err.Error())
	}
	return
}

func createNodeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		body, err := ioutil.ReadAll(r.Body)
		log.Println(string(body))
		if err != nil {
			fmt.Fprintf(w, "Error : %s!", err)
			return
		}
		nodeReq := new(proxy.NodeReq)
		if err = json.Unmarshal(body, nodeReq); err != nil {
			fmt.Fprintf(w, "Error : %s ", err)
			return
		}
		engine, err := proxy.MakeGilmour("127.0.0.1:6379")
		if err != nil {
			fmt.Fprintf(w, "Error : %s!", err)
			return
		}
		node, err := proxy.CreateNode(nodeReq, engine)
		if err != nil {
			fmt.Fprintf(w, "Error : %s!", err)
			return
		}
		if err = node.Start(); err != nil {
			fmt.Fprintf(w, "Error : %s!", err)
			return
		}

		go proxy.NodeWatchdog(node)

		response := node.FormatResponse()
		js, err := json.Marshal(response)
		if err != nil {
			fmt.Fprintf(w, "Error : %s!", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err = w.Write(js); err != nil {
			log.Println(err.Error())
		} else {
			http.NotFound(w, r)
		}
	}
	return
}

// Delete Node
func deleteNodeHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["id"]
	node, err := getNode(id)
	if err != nil {
		logWriterError(w, err)
		return
	}

	if err = proxy.DeleteNode(node); err != nil {
		logWriterError(w, err)
		return
	}

	js := formatResponse("status", "ok")
	w.Header().Set("Content-Type", "application/json")
	if _, err = w.Write(js.([]byte)); err != nil {
		log.Println(err.Error())
	}
}

// POST /nodes/:id/services
func addServicesHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["id"]
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		fmt.Fprintf(w, "Error : %s!", err.Error())
	}
	service := new(proxy.ServiceMap)
	err = json.Unmarshal(body, service)
	if err != nil {
		fmt.Fprintf(w, "Error : %s!", err.Error())
	}
	node, err := getNode(id)
	if err != nil {
		logWriterError(w, err)
		return
	}
	for topic, value := range *service {
		err := node.AddService(topic, value)
		status := setResponseStatus(err)
		js := formatResponse("status", status)
		w.Header().Set("Content-Type", "application/json")
		if _, err = w.Write(js.([]byte)); err != nil {
			log.Println(err.Error())
		}
	}
	return
}

// GET /nodes/{id} getting details of an existing node
func getServicesHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["id"]
	node, err := getNode(id)
	if err != nil {
		logWriterError(w, err)
		return
	}

	response, err := node.GetServices()
	if err != nil {
		logWriterError(w, err)
		return
	}
	js := formatResponse("services", response)
	w.Header().Set("Content-Type", "application/json")
	if _, err = w.Write(js.([]byte)); err != nil {
		//	log.Println(err.EdHat RHCE, RHCSArror())
	}
	return
}

// POST /nodes/:id/slots
func addSlotsHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["id"]
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		fmt.Fprintf(w, "Error : %s!", err.Error())
	}
	slot := new(proxy.Slot)
	err = json.Unmarshal(body, slot)
	if err != nil {
		fmt.Fprintf(w, "Error : %s!", err.Error())
	}
	node, err := getNode(id)
	if err != nil {
		logWriterError(w, err)
		return
	}
	err = node.AddSlot(*slot)
	status := setResponseStatus(err)
	js := formatResponse("status", status)
	w.Header().Set("Content-Type", "application/json")
	if _, err = w.Write(js.([]byte)); err != nil {
		log.Println(err.Error())
	}
	return
}

// GET /nodes/:id/slots
func getSlotsHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["id"]
	node, err := getNode(id)
	if err != nil {
		logWriterError(w, err)
		return
	}
	response, err := node.GetSlots()
	if err != nil {
		logWriterError(w, err)
		return
	}
	js := formatResponse("slots", response)
	w.Header().Set("Content-Type", "application/json")
	if _, err = w.Write(js.([]byte)); err != nil {
		log.Println(err.Error())
	}
	return
}

func RequestServiceHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["id"]
	node, err := getNode(id)
	if err != nil {
		logWriterError(w, err)
		return
	}
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		logWriterError(w, err)
		return
	}
	serviceRequest := new(proxy.Request)
	err = json.Unmarshal(body, &serviceRequest)
	if err != nil {
		logWriterError(w, err)
		return
	}

	response := node.RequestService(*serviceRequest)
	data, err := json.Marshal(response)
	if err != nil {
		logWriterError(w, err)
		return
	}
	w.Write(data)
}

func main() {
	proxy.InitNodeMap()

	r := mux.NewRouter()
	log.Println("listening...")
	r.HandleFunc("/nodes", createNodeHandler)
	r.HandleFunc("/nodes/{id}", deleteNodeHandler).Methods("DELETE")

	r.HandleFunc("/request/{id}", RequestServiceHandler).Methods("POST")

	r.HandleFunc("/nodes/{id}/services", getServicesHandler).Methods("GET")
	r.HandleFunc("/nodes/{id}/services", addServicesHandler).Methods("POST")

	r.HandleFunc("/nodes/{id}/slots", addSlotsHandler).Methods("POST")
	r.HandleFunc("/nodes/{id}/slots", getSlotsHandler).Methods("GET")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Println(err.Error())
	}
}
