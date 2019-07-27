package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"text/template"
	"time"

	"cloud.google.com/go/datastore"
	"google.golang.org/api/iterator"
)

const projectID = "res-log"

func getMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", homeElm)
	mux.HandleFunc("/r", receive)
	mux.HandleFunc("/r/", receive)
	mux.Handle("/l", http.NotFoundHandler())
	mux.HandleFunc("/l/", resourcesView)
	mux.HandleFunc("/cron/daily", dailyView)
	mux.Handle("/task/process_hook", authDecor(http.HandlerFunc(processHookView)))
	mux.Handle("/task/save_resource", authDecor(http.HandlerFunc(saveResourceView)))
	mux.Handle("/task/purge_before", authDecor(http.HandlerFunc(purgeBeforeView)))
	mux.Handle("/task/purge_step", authDecor(http.HandlerFunc(purgeStepView)))
	return mux
}

var elmTmpl = template.Must(template.ParseFiles("templates/index_elm.html"))

func homeElm(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if err := elmTmpl.Execute(w, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func receive(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	err := processBody(ctx, r.Body)
	if err != nil {
		log.Printf("failed to process request with error: %v", err)
	}
	w.Header().Add("X-Application-SHA256", AppKey256)
	fmt.Fprintf(w, "OK")
}

func processBody(ctx context.Context, in io.Reader) error {
	r, err := pack(NewCapReader(8*1024*1024, in)) //8MB arbitrary limit
	if err != nil {
		return err
	}
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	//ctx.Infof("processed data long %d", len(data))
	processHookLater(ctx, data)
	return nil
}

type hookDataAttr struct {
	ID   interface{} `json:"id"`
	Href string      `json:"href"`
}

type hookStruct struct {
	EventType string        `json:"event_type"`
	Resource  string        `json:"resource"`
	Created   string        `json:"created"`
	Data      *hookDataAttr `json:"data"`
}

func processHook(ctx context.Context, in io.Reader) error {
	r, err := unpack(in)
	if err != nil {
		log.Printf("abandon processHook failed to unpack data: %v", err)
		return nil
	}
	var events []*hookStruct
	dec := json.NewDecoder(r)
	err = dec.Decode(&events)
	if err != nil {
		log.Printf("abandon processHook failed to decode json: %v", err)
		return nil
	}
	for _, v := range events {
		saveResourceLater(ctx, v)
		/*
			task, err := saveResourceLater.Task(v)
			if err != nil {
				return err
			}
			// set retry options for the task
			// min/max back off of 5 mins means
			// we will retry every 5 minutes
			// 20 times
			// after which point it will be abandoned
			ropt := taskqueue.RetryOptions{
				RetryLimit: 20,
				MinBackoff: 5 * time.Minute,
				MaxBackoff: 5 * time.Minute,
			}
			task.RetryOptions = &ropt
			_, err = taskqueue.Add(ctx, task, "")
			if err != nil {
				return err
			}*/
	}
	return nil
}

//var processHookLater = delay.Func("processHookKey", processHook)

//Resource is our basic model representing the REST resource that we save to datastore
type Resource struct {
	URI       string `datastore:"Uri"`
	HookDate  string `datastore:",noindex"`
	Data      []byte `datastore:",noindex"`
	FetchDate time.Time
	Sha1      string `datastore:",noindex"`
}

const jsLayout = "2006-01-02T15:04:05Z"

//CountingWriter keeps track of the number of bytes written
type CountingWriter struct {
	Written int
	w       io.Writer
}

//Write implements io.Writer
func (cw *CountingWriter) Write(data []byte) (int, error) {
	num, err := cw.w.Write(data)
	cw.Written = cw.Written + num
	return num, err
}

//WriteString convenience wrapper
func (cw *CountingWriter) WriteString(str string) (int, error) {
	return cw.Write([]byte(str))
}

//NewCountingWriter returns new instance of counting writer
func NewCountingWriter(out io.Writer) *CountingWriter {
	return &CountingWriter{
		0, out,
	}
}

//ErrCapReached is returned by CapReader if the limit (cap) was reached
var ErrCapReached = errors.New("data read cap was reached")

//CapReader keeps track of the number of bytes read
type CapReader struct {
	numRead int
	cap     int
	r       io.Reader
}

//Read implements io.Reader
func (cr *CapReader) Read(p []byte) (int, error) {
	num, err := cr.r.Read(p)
	cr.numRead = cr.numRead + num
	if cr.numRead > cr.cap {
		return num, ErrCapReached
	}
	return num, err
}

//NewCapReader returns new instance of cap reader
func NewCapReader(limit int, in io.Reader) *CapReader {
	return &CapReader{
		cap: limit,
		r:   in,
	}
}

//WriteAsJSON writes this resource to Writer as JSON
func (r *Resource) WriteAsJSON(out io.Writer) (int, error) {
	buf := NewCountingWriter(out)
	buf.WriteString(`{"fetchdate":`)
	data, err := json.Marshal(r.FetchDate.Format(jsLayout))
	if err != nil {
		return 0, err
	}
	buf.Write(data)
	buf.WriteString(`,"hookdate":"`)
	buf.WriteString(r.HookDate)
	buf.WriteString(`","sha1":"`)
	buf.WriteString(r.Sha1)
	buf.WriteString(`","resource":`)
	if len(r.Data) > 0 {
		_, err := unpackTo(buf, bytes.NewBuffer(r.Data))
		if err != nil {
			return 0, err
		}
	} else {
		buf.WriteString("null")
	}
	buf.WriteString("}")
	return buf.Written, nil
}

//MarshalJSON implements the json.Marshaller
func (r *Resource) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	if _, err := r.WriteAsJSON(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func saveResource(c context.Context, hook *hookStruct) (err error) {
	var uriBuf bytes.Buffer
	uriBuf.WriteString(hook.Resource)
	uriBuf.WriteString("/")
	switch t := hook.Data.ID.(type) {
	case int:
		uriBuf.WriteString(strconv.Itoa(t))
	case float64:
		uriBuf.WriteString(strconv.Itoa(int(t)))
	case string:
		uriBuf.WriteString(t)
	default:
		return fmt.Errorf("Unexpected type for Hook.Data.ID: %T", t)
	}
	//fetch the resource
	client := http.DefaultClient
	req, err := http.NewRequest("GET", hook.Data.Href, nil)
	if err != nil {
		log.Printf("failed to build GET request for: %s", hook.Data.Href)
		return
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-Application-Key", cfg.AppKey)
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("failed to fetch: %s", hook.Data.Href)
		return
	}
	defer resp.Body.Close()

	//first reader json verif
	origBuf := new(bytes.Buffer)
	firstR := io.TeeReader(NewCapReader(8*1024*1024, resp.Body), origBuf) //arbitrary 8MB limit
	//before packing calc the sha1 - second reader
	shaw := sha1.New()
	shar := io.TeeReader(firstR, shaw)
	//pack the response returns a final reader
	pr, err := pack(shar)
	if err != nil {
		if err == ErrCapReached {
			log.Printf("cap reached when reading response, abandon: %s", hook.Data.Href)
			err = nil
			return
		}
		log.Printf("failed to pack: %s", hook.Data.Href)
		return
	}
	pdata, err := ioutil.ReadAll(pr)
	if err != nil {
		log.Printf("failed to read packed: %s", hook.Data.Href)
		return
	}
	if len(pdata) > MaxDataStoreByteSize {
		log.Printf(
			"compressed resource is too large %d abandond: %s",
			len(pdata),
			hook.Data.Href)
		err = nil
		return
	}
	//now verify that this is ok json
	var someJSON map[string]interface{}
	dec := json.NewDecoder(origBuf)
	if derr := dec.Decode(&someJSON); derr != nil {
		log.Printf("failed to properly decode json")
		return derr
	}
	//create and save
	r := Resource{
		URI:       uriBuf.String(),
		HookDate:  hook.Created,
		Data:      pdata,
		FetchDate: time.Now().UTC(),
		Sha1:      hex.EncodeToString(shaw.Sum(nil))}

	dsClient, err := datastore.NewClient(c, projectID)
	if err != nil {
		log.Printf("unable to create Datastore client %v", err)
		return
	}

	_, err = dsClient.Put(c, datastore.IncompleteKey("resource", nil), &r)
	if err != nil {
		log.Printf("unable to store resource %#v", r)
		return
	}
	return
}

//MaxDataStoreByteSize is the largest size a blob in DS can have
const MaxDataStoreByteSize = 1048576

//var saveResourceLater = delay.Func("saveResourceKey", saveResource)

func getURLPart(prefix, urlpath string, idx int) (r string) {
	if len(urlpath) <= len(prefix) {
		return
	}
	s := urlpath[len(prefix):]
	p := strings.Split(s, "/")
	if len(p) > idx {
		r = p[idx]
	}
	return
}

func isPrivate(res string) bool {
	private := []string{"documents", "refunds", "payments", "agents"}
	r := strings.ToLower(strings.TrimSpace(res))
	for _, v := range private {
		if r == v {
			return true
		}
	}
	return false
}

//MaxRespSize is maximum response size we are willing to return
const MaxRespSize = 30 * 1024 * 1024 //30MB arbitrary arrived at via 500 errors

func resourcesView(w http.ResponseWriter, r *http.Request) {
	//enable the CORS preflight wonder used by browsers
	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.Header().Add("Access-Control-Allow-Methods", "POST, GET, OPTIONS, HEAD")
	for _, value := range r.Header["Access-Control-Request-Headers"] {
		w.Header().Add("Access-Control-Allow-Headers", value)
	}
	w.Header().Add("Access-Control-Max-Age", "3600")
	//end of CORS
	//response content type header
	w.Header().Add("content-type", "application/json")

	//query
	restype := getURLPart("/l/", r.URL.Path, 0)
	resid := getURLPart("/l/", r.URL.Path, 1)
	if resid == "" || restype == "" {
		http.Error(w, "missing resource type or ID", http.StatusBadRequest)
		return
	}
	if isPrivate(restype) {
		http.Error(w, "Not Authorized", http.StatusForbidden)
		return
	}
	c := r.Context()
	str := bytes.NewBufferString(restype)
	str.WriteString("/")
	str.WriteString(resid)
	q := datastore.NewQuery("resource").Filter("Uri =", str.String()).Order("-FetchDate")
	//iterate query and write it to response up to a limit
	var (
		totalBytes int64
		isFirst    = true
	)
	dsClient, err := datastore.NewClient(c, projectID)
	if err != nil {
		// TODO: Handle error.
		log.Printf("Failed to create a datastore client %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	t := dsClient.Run(c, q)
	out := bufio.NewWriter(w)
	out.WriteString("[")
	for {
		var res Resource
		_, err := t.Next(&res)
		if err == iterator.Done {
			break
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !isFirst {
			out.WriteString(",")
		} else {
			isFirst = false
		}
		size, err := res.WriteAsJSON(out)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		totalBytes = totalBytes + int64(size)
		if totalBytes >= MaxRespSize {
			break
		}
	}
	out.WriteString("]")
	out.Flush()
}

//PurgeInput represents the data struct for purge operation
type PurgeInput struct {
	Before string
}

func purgeBefore(ctx context.Context, when time.Time, encCursor string) (err error) {
	var (
		stop      bool
		keys      []*datastore.Key
		newCursor string
	)
	q := datastore.NewQuery("resource").Filter("FetchDate <", when).KeysOnly()
	if encCursor != "" {
		cursor, err := datastore.DecodeCursor(encCursor)
		if err == nil {
			q = q.Start(cursor)
		}
	} else {
		log.Printf("Starting purge of anything older than %v", when)
	}

	// Iterate over the results.
	dsClient, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		log.Printf("unable to create Datastore client %v", err)
		return
	}

	t := dsClient.Run(ctx, q)
	for i := 0; i < 100; i++ {
		key, err := t.Next(nil)
		if err == iterator.Done {
			stop = true
			break
		}
		if err != nil {
			log.Printf("fetching next Key: %v", err)
			return err
		}
		//do something with it
		keys = append(keys, key)
	}

	// Get updated cursor and store it for next time.
	if cursor, err := t.Cursor(); err == nil {
		newCursor = cursor.String()
	}

	err = dsClient.DeleteMulti(ctx, keys)
	if err != nil {
		log.Printf("trouble with multi delete: %v", err)
		return err
	}
	if !stop {
		purgeStepLater(ctx, newCursor)
	}
	return nil
}

func dailyView(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()
	t := time.Now().UTC().Add(-92 * 24 * time.Hour)
	purgeBeforeLater(ctx, t)
	w.Header().Add("content-type", "application/json")
	fmt.Fprintf(w, "\"OK\"")
}
