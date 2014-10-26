package main

import (
	"code.google.com/p/portaudio-go/portaudio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"time"
)

func main() {
	statusChan := parallel()
	fightingChan := make(chan string)
	go func(audioChan chan string) {
		play("boss.aif", audioChan)
	}(fightingChan)
	fmt.Println(<-statusChan)
	fightingChan <- "stop"
	fanfareChan := make(chan string)
	go func(audioChan chan string) {
		play("fanfare.aif", audioChan)
	}(fanfareChan)
	time.Sleep(10 * time.Second)
	fanfareChan <- "stop"
}

func parallel() <-chan string {
	statusChan := make(chan string)
	go func() {
		cmd := exec.Command("./heavy_task")
		hello, err := cmd.Output()
		if err != nil {
			fmt.Errorf("%s", err)
		}
		statusChan <- string(hello)
	}()
	return statusChan
}

func play(fileName string, finishedChan chan string) {
	//if len(os.Args) < 2 {
	//		fmt.Println("missing required argument:  input file name")
	//		return
	//	}
	fmt.Println("Playing.  Press Ctrl-C to stop.")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)

	f, err := os.Open(fileName)
	chk(err)
	defer f.Close()

	id, data, err := readChunk(f)
	chk(err)
	if id.String() != "FORM" {
		fmt.Println("bad file format")
		return
	}
	_, err = data.Read(id[:])
	chk(err)
	if id.String() != "AIFF" {
		fmt.Println("bad file format")
		return
	}
	var c commonChunk
	var audio io.Reader
	for {
		id, chunk, err := readChunk(data)
		if err == io.EOF {
			break
		}
		chk(err)
		switch id.String() {
		case "COMM":
			chk(binary.Read(chunk, binary.BigEndian, &c))
		case "SSND":
			chunk.Seek(8, 1) //ignore offset and block
			audio = chunk
		default:
			fmt.Printf("ignoring unknown chunk '%s'\n", id)
		}
	}

	//assume 44100 sample rate, mono, 32 bit

	portaudio.Initialize()
	defer portaudio.Terminate()
	out := make([]int32, 8192)
	stream, err := portaudio.OpenDefaultStream(0, 1, 44100, len(out), &out)
	chk(err)
	defer stream.Close()

	chk(stream.Start())
	defer stream.Stop()
	for remaining := int(c.NumSamples); remaining > 0; remaining -= len(out) {
		if len(out) > remaining {
			out = out[:remaining]
		}
		err := binary.Read(audio, binary.BigEndian, out)
		if err == io.EOF {
			break
		}
		chk(err)
		chk(stream.Write())
		select {
		case <-finishedChan:
			return
		case <-sig:
			fmt.Println("Process was killed!")
			os.Exit(1)
		default:
		}
	}

}

func readChunk(r readerAtSeeker) (id ID, data *io.SectionReader, err error) {
	_, err = r.Read(id[:])
	if err != nil {
		return
	}
	var n int32
	err = binary.Read(r, binary.BigEndian, &n)
	if err != nil {
		return
	}
	off, _ := r.Seek(0, 1)
	data = io.NewSectionReader(r, off, int64(n))
	_, err = r.Seek(int64(n), 1)
	return
}

type readerAtSeeker interface {
	io.Reader
	io.ReaderAt
	io.Seeker
}

type ID [4]byte

func (id ID) String() string {
	return string(id[:])
}

type commonChunk struct {
	NumChans      int16
	NumSamples    int32
	BitsPerSample int16
	SampleRate    [10]byte
}

func chk(err error) {
	if err != nil {
		panic(err)
	}
}
