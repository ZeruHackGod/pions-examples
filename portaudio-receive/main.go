package main

import (
	"syscall"
	"os/signal"
	"fmt"
	"io"
	"os"

	"bufio"
	"encoding/base64"

	"github.com/pions/webrtc"
	// "github.com/pions/webrtc/examples/gstreamer-receive/gst"
	"github.com/gordonklaus/portaudio"
	"gopkg.in/hraban/opus.v2"
	"github.com/pions/webrtc/pkg/ice"
)

var ctrlc = make(chan os.Signal)

func main() {
	reader := bufio.NewReader(os.Stdin)
	rawSd, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		panic(err)
	}

	fmt.Println("")
	sd, err := base64.StdEncoding.DecodeString(rawSd)
	if err != nil {
		panic(err)
	}

	/* Everything below is the pion-WebRTC API, thanks for using it! */

	// Setup the codecs you want to use.
	// We'll use the default ones but you can also define your own
	webrtc.RegisterDefaultCodecs()

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.New(webrtc.RTCConfiguration{
		IceServers: []webrtc.RTCIceServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	const channels = 1
	const sampleRate = 16000
	const pcmsize = 320

	portaudio.Initialize()
	defer portaudio.Terminate()
	pcm := make([]int16, pcmsize)

	player, err := portaudio.OpenDefaultStream(0, 1, sampleRate, len(pcm), &pcm)
	if err != nil {
		panic(err)
	}
	defer player.Close()
	signal.Notify(ctrlc, os.Interrupt, syscall.SIGTERM)
	go cleanup(player)

	player.Start()
	if err != nil {
		panic(err)
	}
	defer player.Stop()

	// Set a handler for when a new remote track starts, this handler creates a gstreamer pipeline
	// for the given codec
	peerConnection.OnTrack = func(track *webrtc.RTCTrack) {
		codec := track.Codec
		fmt.Printf("Track has started, of type %d: %s \n", track.PayloadType, codec.Name)
		// pipeline := gst.CreatePip	eline(codec.Name)
		// pipeline.Start()
		// pcm := make([]int16, pcmsize)

		dec, err := opus.NewDecoder(sampleRate, channels)
		if err != nil {
			panic(err)
		}

		for {
			p := <-track.Packets

			_, err := dec.Decode(p.Payload, pcm)
			if err != nil {
				panic(err)
			}

			player.Write()
			// fmt.Println(pcm)
		}
	}

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange = func(connectionState ice.ConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	}

	// Set the remote SessionDescription
	offer := webrtc.RTCSessionDescription{
		Type: webrtc.RTCSdpTypeOffer,
		Sdp:  string(sd),
	}
	if err := peerConnection.SetRemoteDescription(offer); err != nil {
		panic(err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	// Get the LocalDescription and take it to base64 so we can paste in browser
	fmt.Println(base64.StdEncoding.EncodeToString([]byte(answer.Sdp)))

	select {}
}

func cleanup(player *alsa.PlaybackDevice) {
	// User hit Ctrl-C, clean up
	<-ctrlc
	fmt.Println("Close devices")
	player.Close()
	os.Exit(1)
}
