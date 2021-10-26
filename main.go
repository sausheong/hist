package main

import (
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"errors"
	"image/color"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"text/template"
	"time"

	"github.com/gorilla/mux"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/font"
	"gonum.org/v1/plot/plotter"
)

var dir string
var port string

func init() {
	// get the app directory
	// for Heroku this is /app
	// for Digital Ocean App Platform this is /workspace
	var err error
	dir = os.Getenv("APPDIR")
	if dir == "" {
		// if not set, use the path where the app is run
		dir, err = filepath.Abs(filepath.Dir(os.Args[0]))
		if err != nil {
			log.Println("Cannot get app file dir:", err)
		}
	}

	// get the port to run the web server
	port = os.Getenv("PORT")
	if port == "" {
		port = "8000" // localhost runs on 8000
	}
	// initFonts()
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/", index)
	r.HandleFunc("/make", makeHist)

	r.PathPrefix("/static").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(dir+"/static"))))

	srv := &http.Server{
		Handler:      r,
		Addr:         ":" + port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	log.Println("Starting server at", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}

// front page handle
func index(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles(dir + "/static/index.html")
	t.Execute(w, nil)
}

// histogram-making handler
func makeHist(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(8192)
	var title string
	var err error
	var clr color.Color
	var nbins, width, height int

	for k, v := range r.PostForm {
		if len(v) > 0 {
			switch k {
			case "title":
				title = v[0]
				if title == "" {
					title = "Histogram"
				}
			case "bins":
				nbins, _ = strconv.Atoi(v[0])
				nbins = assignIf(nbins, 25)
			case "width":
				width, _ = strconv.Atoi(v[0])
				width = assignIf(width, 400)
			case "height":
				height, _ = strconv.Atoi(v[0])
				height = assignIf(height, 200)
			case "color":
				clr, err = parseHexColor(v[0])
				if err != nil {
					clr = color.RGBA{R: 255, G: 127, B: 80, A: 255} // default is Coral
				}
			}
		}
	}
	buf := hist(r, title, nbins, width, height, clr)
	data := base64.StdEncoding.EncodeToString(buf.Bytes())
	w.Write([]byte(data))
}

// make a histogram
func hist(r *http.Request, title string, nbins, width, height int, clr color.Color) (buf bytes.Buffer) {
	file, _, err := r.FormFile("csv")
	if err == nil {
		reader := csv.NewReader(file)
		rows, err := reader.ReadAll()
		if err != nil {
			log.Println("Cannot read CSV file:", err)
		}
		length := len(rows)
		data := make([]float64, length)
		for i := 0; i < length; i++ {
			data[i], _ = strconv.ParseFloat(rows[i][0], 64)
		}

		n := len(data)
		vals := make(plotter.Values, n)
		for i := 0; i < n; i++ {
			vals[i] = data[i]
		}

		// start creating the histogram
		plt := plot.New()
		plt.Title.Text = title
		plt.Title.Padding = 25
		hist, err := plotter.NewHist(vals, nbins)
		if err != nil {
			log.Println("Cannot plot:", err)
		}
		hist.FillColor = clr
		plt.Add(hist)

		// write to Writer
		w, _ := font.ParseLength(strconv.Itoa(width))
		h, _ := font.ParseLength(strconv.Itoa(height))
		writerto, err := plt.WriterTo(w, h, "png")
		if err != nil {
			log.Println("Cannot get WriterTo:", err)
		}

		writerto.WriteTo(&buf)
		if err != nil {
			log.Println("Cannot write to image:", err)
		}

	} else {
		log.Println("Cannot parse CSV file:", err)
	}

	return
}

// use value if it's not 0, else use def
func assignIf(value, def int) int {
	if value == 0 {
		return def
	}
	return value
}

// parse the hex color into color.RGBA
// adapted from https://stackoverflow.com/questions/54197913/parse-hex-string-to-image-color
var errInvalidFormat = errors.New("invalid format")

func parseHexColor(s string) (c color.RGBA, err error) {
	c.A = 0xff

	if s == "" || s[0] != '#' {
		return c, errInvalidFormat
	}

	hexToByte := func(b byte) byte {
		switch {
		case b >= '0' && b <= '9':
			return b - '0'
		case b >= 'a' && b <= 'f':
			return b - 'a' + 10
		case b >= 'A' && b <= 'F':
			return b - 'A' + 10
		}
		err = errInvalidFormat
		return 0
	}

	switch len(s) {
	case 7:
		c.R = hexToByte(s[1])<<4 + hexToByte(s[2])
		c.G = hexToByte(s[3])<<4 + hexToByte(s[4])
		c.B = hexToByte(s[5])<<4 + hexToByte(s[6])
	case 4:
		c.R = hexToByte(s[1]) * 17
		c.G = hexToByte(s[2]) * 17
		c.B = hexToByte(s[3]) * 17
	default:
		err = errInvalidFormat
	}
	return
}
