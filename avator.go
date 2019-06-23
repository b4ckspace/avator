package main

import (
	"bytes"
	"crypto/sha256"
	"image/color"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/golang/snappy"
	"github.com/taironas/tinygraphs/draw/spaceinvaders"
)

type cacheKey struct {
	nic  string
	size int
}

var (
	palett = []color.RGBA{
		{38, 84, 124, 255},
		{239, 71, 111, 255},
		{255, 209, 102, 255},
		{6, 214, 160, 255},
	}
	cache      = sync.Map{}
	cacheOrder chan cacheKey
)

func main() {
	listen, ok := os.LookupEnv("LISTEN")
	if !ok {
		listen = ":8080"
	}

	cacheSize, ok := os.LookupEnv("CACHE_SIZE")
	cacheSizeInt, err := strconv.Atoi(cacheSize)
	if !ok || err != nil {
		cacheSizeInt = 1024
	}
	log.Printf("Cache size is %d", cacheSizeInt)
	cacheOrder = make(chan cacheKey, cacheSizeInt)

	http.HandleFunc("/avatar/", avatar)
	log.Printf("Listening on %s\n", listen)
	http.ListenAndServe(listen, nil)
}

func avatar(w http.ResponseWriter, r *http.Request) {
	nic := r.URL.RequestURI()[8:]
	seed := sha256.Sum224([]byte(nic))
	sseed := string(seed[:])

	q := r.URL.Query()
	size := 128
	urlSize := ""
	if s, ok := q["s"]; ok {
		urlSize = s[0]
	} else if s, ok := q["size"]; ok {
		urlSize = s[0]
	}
	if s, err := strconv.Atoi(urlSize); err == nil {
		size = s
	}

	w.Header().Add("content-type", "image/svg+xml")

	key := cacheKey{nic, size}
	ci, ok := cache.Load(key)
	c, ok2 := ci.([]byte)
	if ok && ok2 {
		c := bytes.NewBuffer(c)
		sc := snappy.NewReader(c)
		io.Copy(w, sc)
	} else {
		c := bytes.NewBuffer([]byte{})
		sc := snappy.NewBufferedWriter(c)
		cw := io.MultiWriter(w, sc)
		spaceinvaders.SpaceInvaders(cw, sseed, palett, size)
		go func() {
			sc.Flush()
			cache.Store(key, c.Bytes())
			select {
			case cacheOrder <- key:
			default:
				toDelete := <-cacheOrder
				cache.Delete(toDelete)
				cacheOrder <- key
			}
		}()
	}
}
