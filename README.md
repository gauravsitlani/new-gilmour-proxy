# new-gilmour-proxy
As the library of Gilmour is available just for golang and ruby, what if we want to use it with another language.  
Its an HTTP proxy to make sure we can use the features of Gilmour.
For more details refer to [Gilmour](https://github.com/gilmour-libs/gilmour-e-go)

--------------------------------------------

# Using new-gilmour-proxy
Gilmour proxy listens for http requests on a port it is configured to start with. The following routes are available on the control port V

----------------------------------------------
# The control TCP ports. 

## Create a new upstream Gilmour node.

### :POST /nodes

**Request Body**
```
{
    port: int <the port number this node will listen on>,
    health_check: string <http path at port which responds to health ping.default: /health>,
    slots: [
    {
        topic: string <topic to listen on. can be a wildcard>,
        group: string <optional exclusion group>,
        path: string <http path at port corresponding to the handler for this slot>,
        timeout: int <time after which the proxy times out this call>        
    }, ...
    ],
    services: [
    {
        topic: string <topic to listen on. cannot be a wildcard>,
        group: string <mandatory exclusion group>,
        path: string <http path at port corresponding to the handler for this service>,
        timeout: int <time after which the proxy times out this call>  
        }, ...
        ],
}
```

Before returning a response, the proxy performs the following steps:
1. Send a health check ping on the health_check path. Return error if not successful
2. Create subscriptions for the signals and slots provided

**Response Body**
```
{
    id: string <a uuid identifying this node. this needs to be reused for further requests>,
    status: string <status of the node - "ok", "unavailable" or "dirty">
}
```

1. The *id* in the above response has to be stored somewhere, because this *id* is useful for the managing the node's services, slots and the node itself.
2. The *health_check* is the path for health check. The proxy will ping this path after every 10 seconds, to monitor the availability of the node. If the node fails to respond to the health check pings. It is marked as *unavailable*. All subscriptions corresponding to this node will be removed. The subscriptions will be setup again once the node starts responding to the health checks. If the listener for the node for port itself cannot be validated, the node is marked as "dirty" and all activity related to the node is stopped.

## Get Details of the existing node

### :GET /nodes/:id

**Response**
```
{
    id: string <node uuid>,
    port: int <the port number this node will listen on>,
    health_check: string <http path at listen_sock which responds to health ping>,
    slots: [
    {
        topic: string <topic>,
        group: string <exclusion group>,
        path: string <handler http path for this slot>,
        timeout: int <time after which the proxy times out this call>
    }, ...
    ],
    services: [
    {
        topic: string <topic>,
        group: string <exclusion group>,
        path: string <handler http pathfor this service>,
        timeout: int <time after which the proxy times out this call>
    }, ...
    ],
    status: string <status of the node - "ok", "unavailable". "dirty">
}
```

## Gracefully remove a node

### :DELETE /nodes/:id

**Response**
```
{
    status:string <'ok' or error message>
}
```
*Notes*
1. This will not affect any running tasks.
2. The node will no longer appear in the list of nodes. All resources allocated to the node will be freed.

## Add a slot subscription

### :POST /nodes/:id/slots

**Request**
```
{
    topic: string <topic>,
    group: string <exclusion group>,
    path: string <handler http path for this slot>,
    timeout: int <time after which the proxy times out this call> 
}
```
**Response**
```
{
    status:string <'ok' or error message>
}
```

## Get list of slot subscriptions

### :GET /nodes/:id/slots

**Response**
```
{
   slots:[
       {
           topic: string <topic>,
           group: string <exclusion group>,
           path: string <handler http path for this slot>,
           timeout: int <time after which the proxy times out this call> 
       }, ......
   ]
}
```

## Remove slot subscription(s)

### :DELETE /nodes/:id/slots?topic=<topic>&path=<path>

**Response**
```
{
    status:string <'ok' or error message>
}
```

*Notes*
1. The path is optional. If it is not provided, all subscriptions corresponding to the topic will be removed.
2. This will not affect any running executions (unless terminated by the node)


## Add a service subscription

### :POST /nodes/:id/services

**Request**
```
{
    topic: string <topic>,
    group: string <exclusion group>,
    path: string <handler http path for this service>,
    timeout: int <time after which the proxy times out this call>
}
```
**Response**
```
{
    status:string <'ok' or error message>
}
```
## Get list of service subscriptions

### :GET /nodes/:id/services

**Response**
```
{
    services: [
    {
        topic: string <topic>,
        group: string <exclusion group>,
        path: string <handler http path for this service>,
        timeout: int <time after which the proxy times out this call>   
        }, ...
        ]
}
```

## Remove service subscription(s)

### :DELETE /nodes/:id/services

**Response**
```
{
status: string <'ok' or error message.>
}
```
*Notes*
1. The path is optional. If it is not provided, all subscriptions corresponding to the topic will be removed.
2. This will not affect any running executions (unless terminated by the node)
3. The get list of service and the slot controls are for monitoring purposes.

------------------------------------------

# The Publish Port
When a node registers with the proxy (or is re-activated), it accepts publish requests from the node on the port which is used to register the proxy. 

## Send a request

### :POST /request/:id/
The :id is the uuid of the node which makes this request

**Request**
```
{
    topic : string <topic to send the request to.>,
    composition : composition_spec,
    message : any <request data>,
    timeout : int <timeout in seconds>
}
```
Either one of topic or composition_spec can be specified. This call will block till the request returns a response or times out.

**Response**
```
{
    messages: [
    {
        data: any <response data>, code: int <response code>
    },...
    ],
    code: int <max response code of all the messages>,
    length: int <number of messages in the response> 
}
```

*Notes*
1. The response is composed of one or more (see batch and parallel compositions) messages. Each message has its own data and code.
2. The code in the top level response body is the maximum of all the codes in the response body. This will also be the http response code.
---------------------------------------------

# The Port Protocol
During registration, every node provides a *port* on which it listens for requests and signals, for its subscriptions. The *health_check* endpoint has already been documented above. This section describes the protocol for calling the http endpoints when a corresponding request or signal is received.

## Service endpoint
These are called for corresponding incoming requests. A *POST* request is made to the endpoint. The request body is
```
{
    topic: string <topic on which this request was made>,
    sender: string <a unique uuid for this request>,
    data: any <request data>,
    timeout: int <timeout that this service was setup with>
}
```

*Notes*
1. If the request handler has not finished executing before the timeout, the proxy will send a timeout error code back to the client. Before it does this, it will also close the connection, so that a response write will fail.
2. The response body of the handler will be sent unmodified to the client in the response data

## Slot endpoint
These are called for corresponding incoming requests. A *POST* request is made to the slot endpoint. The request body is
```
{
topic: string <topic on which this request was made>,
sender: string <a unique uuid for this request>,
data: any <request data>,
timeout: int <timeout that this service was setup with>
}
```
