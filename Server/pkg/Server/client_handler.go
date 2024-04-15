package Server

import (
	"Utils"
	"context"
	"errors"
	"log/slog"
	"net"
)

// ClientHandler is a struct that handles a GoExpose client
type ClientHandler struct {
	Conn net.Conn

	exposedTcpPorts map[int]Relay
	exposedUdpPorts map[int]Relay
	proxyPorts      *Portqueue

	logger *slog.Logger
}

// HandleClient is a function that handles a client connection. It creates a new ClientHandler and calls its handle function (blocking).
func HandleClient(ctx context.Context, conn net.Conn, logger *slog.Logger) {
	ch := new(ClientHandler)
	ch.Conn = conn
	ch.exposedTcpPorts = make(map[int]Relay)
	ch.exposedUdpPorts = make(map[int]Relay)
	ch.proxyPorts = NewPortqueue()
	ch.logger = logger
	// handle is a blocking function that handles the client connection
	ch.handle(ctx)
}

// handle is the actual loop that handles a client connection. The server calls this and blocks until the client disconnects.
// It reads frames from the client, digests them, and sends responses back to the client.
// The client connection is closed when the function returns.
// The function creates a child context of root, which is used to synchronize all proxy operations with the GoExpose client that is handled here.
func (c *ClientHandler) handle(ctx context.Context) {
	defer func() {
		_ = c.Conn.Close()
	}()
	// reqChan receives requests from the client as input through a helper goroutine
	reqChan := make(chan *Utils.CTRLFrame, 10)
	// respChan receives responses generated by this client handler as input through the digestFrame function
	respChan := make(chan *Utils.CTRLFrame, 10)
	defer close(reqChan)
	defer close(respChan)

	// clientctx gets terminated once the client connection is closed
	clientctx, cnl := context.WithCancel(ctx)

	go c.readFrames(clientctx, reqChan, cnl)

	for {
		select {
		case <-clientctx.Done():
			return
		case msg := <-reqChan:
			// digest the request from the
			c.logger.Debug("Received frame from client", slog.String("Func", "handle"), "Frame", msg.String())
			c.digestFrame(msg, respChan, cnl)
		case msg := <-respChan:
			// send the response to the client
			c.logger.Debug("Sending response to client", slog.String("Func", "handle"), "Frame", msg.String())
			by, err := Utils.ToByteArray(msg)
			if err != nil {
				c.logger.Error("Error converting frame to byte array", slog.String("Func", "handle"), "Error", err)
				continue
			}
			_, err = c.Conn.Write(by)
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					c.logger.Debug("Client connection closed", slog.String("Func", "handle"))
					cnl()
					return
				}
				c.logger.Debug("Error writing frame to client", slog.String("Func", "handle"), "Error", err)
			}
		}
	}
}

// readFrames is a helper goroutine that reads frames from the client and passes them to the fromclient channel.
// The function returns when the client connection is closed or the context is cancelled.
func (c *ClientHandler) readFrames(ctx context.Context, fromclient chan *Utils.CTRLFrame, cnl context.CancelFunc) {
	defer cnl()
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// read frames from the client and pass them to the fromclient channel
			fr, err := Utils.ReadFrame(c.Conn)
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					c.logger.Debug("Client connection closed", slog.String("Func", "readFrames"))
					return
				} else {
					c.logger.Error("Error reading frame from client", slog.String("Func", "readFrames"), "Error", err)
					return
				}
			}
			fromclient <- fr
		}
	}
}

// digestFrame is a function that processes a frame from the client and sends a response to the client.
// It contains the logic to handle the different types of frames that the client can send.
//
// TODO: continue the rewrite here
func (c *ClientHandler) digestFrame(msg *Utils.CTRLFrame, toclient chan *Utils.CTRLFrame, cnl context.CancelFunc) {
	switch msg.Typ {
	case Utils.CTRLUNPAIR:
		// unpair the client by cancelling the context of this ClientHandler
		cnl()
		return
	case Utils.CTRLEXPOSETCP:
		// Expose the tcp port
	case Utils.CTRLHIDETCP:
		// Hide the tcp port
	case Utils.CTRLEXPOSEUDP:
		// Expose the udp port
	case Utils.CTRLHIDEUDP:
		// Hide the udp port
	}
}
