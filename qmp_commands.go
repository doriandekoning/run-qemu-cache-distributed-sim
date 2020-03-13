package main

import (
	"encoding/json"
	"log"
	"time"
	"strings"
	"net"
	"fmt"
)

// Some stuff to interact with QEMU over QMP (on a unix socket), the code could be cleaned up a lot

type QmpCommand struct {
	Cmd string `json:"execute"`
	Args map[string]interface{} `json:"arguments,omitempty"`
}

type QmpConn struct {
	conn net.Conn
}

type Empty struct {}

type SuccessResponse struct {
	Return Empty `json:"return"`
}

type TimestampResponse struct {
	Seconds int `json:"seconds"`
	Microseconds int `json:"microseconds"`
}

type EventResponse struct {
	Timestamp TimestampResponse `json:"timestamp"`
	Event string `json:"event"`
}

type E820Info struct {
	Start uint64 `json:"start"`
	End   uint64 `json:"end"`
}

type E820InfoResp struct {
	Ranges []*E820Info `json:"return"`
}


func InitQMPConnection(path string) *QmpConn {
	qmpCon, err := net.Dial("unix", path)
	if err != nil {
		log.Fatalf("Unable to connect to qmp on socket")
	}
	_, err = qmpCon.Write(GetQmpCmd("qmp_capabilities", nil)) //TODO do not hardcode memsize is hardcoded to 8G
	if err != nil {
		log.Fatal("Unable to execute qmp command")
	}
	conn := &QmpConn{conn: qmpCon}
	resps := conn.getResponses()
	conn.expectSuccess(resps[0])
	return conn
}

func (conn *QmpConn) Stop() {

	cmd := GetQmpCmd("stop", nil)
	n, err := conn.conn.Write(cmd)
	if err != nil || len(cmd) != n {
		log.Fatal("Unable to execute qmp command")
	}
	resps := conn.getResponses()
	conn.expectSuccess(resps[0])
}

func (conn *QmpConn) Cont() {
	cmd := GetQmpCmd("cont", nil)
	n, err := conn.conn.Write(cmd)
	if err != nil || len(cmd) != n {
		log.Fatal("Unable to execute qmp command")
	}
	resps := conn.getResponses()
	conn.expectSuccess(resps[0])
}

func (conn *QmpConn) pmemsave(start uint64, end uint64, location string) {
	cmd := GetQmpCmd("pmemsave", map[string]interface{}{"val":start, "size":end - start, "filename":location})
	n, err := conn.conn.Write(cmd)
	if err != nil || len(cmd) != n {
		log.Fatal("Unable to execute qmp command")
	}
	resps := conn.getResponses()
	conn.expectSuccess(resps[0])
}

func (conn *QmpConn) SetTraceEvent(name string, enabled bool) {
	cmd := GetQmpCmd("trace-event-set-state", map[string]interface{}{"name":name, "enable":enabled})
	n, err := conn.conn.Write(cmd)
	if err != nil || len(cmd) != n {
		log.Fatal("Unable to execute qmp command")
	}
	resps := conn.getResponses()
	conn.expectSuccess(resps[0])
}


func (conn *QmpConn) InfoRegisters() {
	cmd := GetQmpCmd("info", nil)
	n, err := conn.conn.Write(cmd)
	if err != nil || len(cmd) != n {
		log.Fatal("Unable to execute qmp command")
	}
	conn.expectSuccess(conn.getResponses()[0])
}

func (conn *QmpConn) InfoE820() []*E820Info {
	cmd := GetQmpCmd("e820-info", nil)
	fmt.Println("E802infoning")
	n, err := conn.conn.Write(cmd)
	if err != nil || len(cmd) != n {
		log.Fatal("Unable to execute InfoE820 qmp cmd")
	}
	connResponses := conn.getResponses()
	conn.expectSuccess(connResponses[0])
	resp := E820InfoResp{}
	connResponses = conn.getResponses()
	log.Println(connResponses[0])
	err = json.Unmarshal([]byte(connResponses[0]), &resp)
	if err != nil {
		log.Println(connResponses[0])
		log.Fatal("Unable to unmarshal response: ", err)
	}
	return resp.Ranges
}

func (conn *QmpConn) InfoMemoryDevices() {
	cmd := GetQmpCmd("query-memory-devicefffs", nil)
	n, err := conn.conn.Write(cmd)
	if err != nil || len(cmd) != n {
		log.Fatal("Unable to execute qmp command")
	}
	fmt.Println(conn.getResponses()[0])
}

func (conn *QmpConn) InfoPCIEDevices() {
	cmd := GetQmpCmd("query-pci", nil)
	n, err := conn.conn.Write(cmd)
	if err != nil || len(cmd) != n {
		log.Fatal("Unable to execute command")
	}
	fmt.Println(conn.getResponses()[0])
}

func (conn *QmpConn) EnableTraceEvent() {

}

// func (conn *QmpConn) setTraceEvent(name string, state bool) {
// 	cmd := GetQmpCmd("trace-event-set-state", []string{"name", "enable"}, []interface{name, state})
// }


func GetQmpCmd(cmd string, args map[string]interface{}) []byte{
	qmpCmd := QmpCommand{
		Cmd: cmd,
		Args: args,
	}
	time.Sleep(1*time.Second)
	bytes, err := json.Marshal(qmpCmd)
	if err != nil {
		panic(err)
	}
	log.Println("QPM[SEND]:", string(bytes))
	return bytes
}


func isSuccess(bytes []byte) bool {
	resp := SuccessResponse{}
	err := json.Unmarshal(bytes, &resp)
	return err == nil
}

func (conn *QmpConn) getResponses() []string {
	received := make([]byte,1000)
	n, err := conn.conn.Read(received)
	if err != nil {
		log.Fatal("Unable to receive")
	}
	resp :=  strings.Split(string(received[:n]), "\n")
	for _, r := range resp {
		log.Println("QMP[RECV]:", r)
	}
	return resp
}


func (conn *QmpConn) expectSuccess(resp string) {
	if !isSuccess([]byte(resp)) {
		log.Fatalf("Error while executing command:%s", resp)
	}
}

func (conn *QmpConn) expectEvent(respStr, event string) {
	resp := EventResponse{}
	err := json.Unmarshal([]byte(respStr), &resp)
	if err != nil {
		log.Fatalf("Unexpected response: %s\n", respStr)
	}
	if resp.Event != event {
		log.Fatalf("unexpected event:%s, %s\n", resp.Event, respStr)
	}
}



