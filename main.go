package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"./proxy"
	"github.com/gorilla/mux"
)

func createNodeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			fmt.Fprintf(w, "Error : %s!", proxy.LogError(err))
		}
		nodeReq := new(proxy.NodeReq)
		if err = json.Unmarshal(body, nodeReq); err != nil {
			fmt.Fprintf(w, "Error : %s ", proxy.LogError(err))
		}
		engine, err := proxy.MakeGilmour("127.0.0.1:6379")
		if err != nil {
			fmt.Fprintf(w, "Error : %s!", proxy.LogError(err))
		}
		node, err := proxy.CreateNode(nodeReq, engine)
		if err != nil {
			fmt.Fprintf(w, "Error : %s!", proxy.LogError(err))
		}
		if err = node.Start(); err != nil {
			fmt.Fprintf(w, "Error : %s!", proxy.LogError(err))
		}

		go proxy.NodeWatchdog(node)

		response := node.FormatResponse()
		js, err := json.Marshal(response)
		if err != nil {
			fmt.Fprintf(w, "Error : %s!", proxy.LogError(err))
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

func getNode(id string) (node *proxy.Node, err error) {
	nm := proxy.GetNodeMap()
	node, err = nm.Get(proxy.NodeID(id))
	return
}

func formatResponse(key string, value interface{}) interface{} {
	js, err := json.Marshal(map[string]interface{}{key: value})
	if err != nil {
		proxy.LogError(err)
		js = []byte(err.Error())
	}
	return js
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


func main() {
	r := mux.NewRouter()
	log.Println("listening...")
	r.HandleFunc("/nodes", createNodeHandler)
	r.HandleFunc("/nodes/{id}", deleteNodeHandler).Methods("DELETE")
	http.Handle("/", r)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Println(err.Error())
	}
}
