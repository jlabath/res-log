package library

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"text/template"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/delay"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/urlfetch"
	"google.golang.org/appengine/user"
)

func init() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", homeElm)
	mux.HandleFunc("/legacy/", home)
	mux.HandleFunc("/r", receive)
	mux.HandleFunc("/r/", receive)
	mux.Handle("/l", http.NotFoundHandler())
	mux.HandleFunc("/l/", resourcesView)
	mux.HandleFunc("/admin/", adminElm)
	mux.HandleFunc("/purge/", purgeView)
	http.Handle("/", mux)
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

var adminTmpl = template.Must(template.ParseFiles("templates/admin.html"))

func adminElm(w http.ResponseWriter, r *http.Request) {
	if err := adminTmpl.Execute(w, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

var homeTmpl = template.Must(template.ParseFiles("templates/index.html"))

func home(w http.ResponseWriter, r *http.Request) {
	if err := homeTmpl.Execute(w, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func receive(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	err := processBody(ctx, r.Body)
	if err != nil {
		log.Errorf(ctx, "failed to process request with error: %v", err)
	}
	w.Header().Add("X-Application-SHA256", AppKey256)
	fmt.Fprintf(w, "OK")
}

func processBody(ctx context.Context, in io.Reader) error {
	r, err := pack(in)
	if err != nil {
		return err
	}
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	//ctx.Infof("processed data long %d", len(data))
	processHookLater.Call(ctx, data)
	return nil
}

type hookDataAttr struct {
	ID   interface{} `json:id`
	Href string      `json:href`
}

type hookStruct struct {
	EventType string        `json:event_type`
	Resource  string        `json:resource`
	Created   string        `json:created`
	Data      *hookDataAttr `json:data`
}

func processHook(ctx context.Context, data []byte) (err error) {
	r, err := unpack(bytes.NewBuffer(data))
	if err != nil {
		log.Errorf(ctx, "failed to unpack data: %v", err)
		return
	}
	var events []*hookStruct
	dec := json.NewDecoder(r)
	err = dec.Decode(&events)
	if err != nil {
		log.Errorf(ctx, "failed to decode json: %v", err)
		return
	}
	for _, v := range events {
		saveResourceLater.Call(ctx, v)
	}
	return
}

var processHookLater = delay.Func("processHookKey", processHook)

//Resource is our basic model representing the REST resource that we save to datastore
type Resource struct {
	URI       string `datastore:"Uri"`
	HookDate  string `datastore:",noindex"`
	Data      []byte `datastore:",noindex"`
	FetchDate time.Time
	Sha1      string `datastore:",noindex"`
}

const jsLayout = "2006-01-02T15:04:05Z"

//ToJSON converts this Resource into JSONResource
func (r *Resource) ToJSON() (*JSONResource, error) {
	v := JSONResource{
		FetchDate: r.FetchDate.Format(jsLayout),
		HookDate:  r.HookDate,
		Sha1:      r.Sha1}
	if len(r.Data) > 0 {
		ur, err := unpack(bytes.NewBuffer(r.Data))
		if err != nil {
			return nil, err
		}
		buf, err := ioutil.ReadAll(ur)
		if err != nil {
			return nil, err
		}
		v.Resource = json.RawMessage(buf)
	} else {
		v.Resource = json.RawMessage([]byte("null"))
	}
	return &v, nil
}

//JSONResource is the REST suitable representation of a Resource
//this is what's consumed by the User Interface javascript
type JSONResource struct {
	FetchDate string          `json:"fetchdate"`
	HookDate  string          `json:"hookdate"`
	Sha1      string          `json:"sha1"`
	Resource  json.RawMessage `json:"resource"`
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
	client := urlfetch.Client(c)
	req, err := http.NewRequest("GET", hook.Data.Href, nil)
	if err != nil {
		log.Errorf(c, "failed to build GET request for: %s", hook.Data.Href)
		return
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-Application-Key", cfg.AppKey)
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf(c, "failed to fetch: %s", hook.Data.Href)
		return
	}
	//first reader json verif
	origBuf := new(bytes.Buffer)
	firstR := io.TeeReader(resp.Body, origBuf)
	//before packing calc the sha1 - second reader
	shaw := sha1.New()
	shar := io.TeeReader(firstR, shaw)
	//pack the response returns a final reader
	pr, err := pack(shar)
	if err != nil {
		log.Errorf(c, "failed to pack: %s", hook.Data.Href)
		return
	}
	pdata, err := ioutil.ReadAll(pr)
	if err != nil {
		log.Errorf(c, "failed to read packed: %s", hook.Data.Href)
		return
	}
	//now verify that this is ok json
	var someJSON map[string]interface{}
	dec := json.NewDecoder(origBuf)
	if derr := dec.Decode(&someJSON); derr != nil {
		log.Errorf(c, "failed to properly decode json")
		return derr
	}
	//create and save
	r := Resource{
		URI:       uriBuf.String(),
		HookDate:  hook.Created,
		Data:      pdata,
		FetchDate: time.Now().UTC(),
		Sha1:      hex.EncodeToString(shaw.Sum(nil))}
	_, err = datastore.Put(c, datastore.NewIncompleteKey(c, "resource", nil), &r)
	if err != nil {
		log.Errorf(c, "unable to store resource %#v", r)
		return
	}
	return
}

var saveResourceLater = delay.Func("saveResourceKey", saveResource)

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

func resourcesView(w http.ResponseWriter, r *http.Request) {
	//enable the CORS preflight wonder used by browsers
	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.Header().Add("Access-Control-Allow-Methods", "POST, GET, OPTIONS, HEAD")
	for _, value := range r.Header["Access-Control-Request-Headers"] {
		w.Header().Add("Access-Control-Allow-Headers", value)
	}
	w.Header().Add("Access-Control-Max-Age", "3600")
	//end of CORS
	restype := getURLPart("/l/", r.URL.Path, 0)
	resid := getURLPart("/l/", r.URL.Path, 1)
	if resid == "" || restype == "" {
		http.Error(w, "missing resource type or ID", http.StatusBadRequest)
		return
	}
	c := appengine.NewContext(r)
	str := bytes.NewBufferString(restype)
	str.WriteString("/")
	str.WriteString(resid)
	q := datastore.NewQuery("resource").Filter("Uri =", str.String()).Order("-FetchDate")
	var resources []*Resource
	jsonResources := make([]*JSONResource, 0, 10)
	if _, err := q.GetAll(c, &resources); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, v := range resources {
		x, xerr := v.ToJSON()
		if xerr != nil {
			http.Error(w, xerr.Error(), http.StatusInternalServerError)
			return
		}
		jsonResources = append(jsonResources, x)
	}
	w.Header().Add("content-type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(jsonResources); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type PurgeInput struct {
	Before string
}

func purgeView(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := appengine.NewContext(r)
	if !user.IsAdmin(ctx) {
		http.Error(w, "Not Authorized", http.StatusUnauthorized)
		return
	}
	var pi PurgeInput
	err := json.NewDecoder(r.Body).Decode(&pi)
	if err != nil {
		log.Errorf(ctx, "failed to process request with error: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	t, err := time.Parse("2006-01-02", pi.Before)
	if err != nil {
		log.Errorf(ctx, "failed to process request with error: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	purgeBeforeLater.Call(ctx, t, "")
	w.Header().Add("content-type", "application/json")
	fmt.Fprintf(w, "\"OK\"")
}

var purgeBeforeLater = delay.Func("purgeBefore", purgeBefore)

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
	}

	// Iterate over the results.
	t := q.Run(ctx)
	for i := 0; i < 100; i++ {
		key, err := t.Next(nil)
		if err == datastore.Done {
			stop = true
			break
		}
		if err != nil {
			log.Errorf(ctx, "fetching next Key: %v", err)
			return err
			break
		}
		//do something with it
		keys = append(keys, key)
	}

	// Get updated cursor and store it for next time.
	if cursor, err := t.Cursor(); err == nil {
		newCursor = cursor.String()
	}

	err = datastore.DeleteMulti(ctx, keys)
	if err != nil {
		log.Errorf(ctx, "trouble with multi delete: %v", err)
		return err
	}
	if !stop {
		f := delay.Func("purge_step "+newCursor, purgeBefore)
		f.Call(ctx, when, newCursor)
	}
	return nil
}
