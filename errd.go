package errd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

//ErrHandler Here.
type ErrHandler struct {
	topCtx        context.Context
	Ctx           context.Context
	ctxCancel     context.CancelFunc
	errChan       chan error
	errRespChan   chan bool
	SigTermChan   chan os.Signal
	stopChan      chan bool
	HandleFunc    func(error) bool
	running       bool
	SigTerm       bool
	verbose       bool
	logging       bool
	Logger        *log.Logger
	verboseLogger *log.Logger
}

//ErrConfig Here.
type ErrConfig struct {
	ctx       context.Context
	ctxCancel bool
	Logger    *log.Logger
}

//Error Here.
type Error struct {
	Code    int
	Value   error
	Comment string
}

func (ee Error) Error() string {
	return fmt.Sprintf("%v: %v", ee.Code, ee.Error, ee.Comment)
}

//NewErrHandler Here. Creates new ErrHandler with defaults.
func NewErrHandler(ctx context.Context) *ErrHandler {
	ErrHandler := ErrHandler{
		topCtx:      ctx,
		errChan:     make(chan error, 1),
		errRespChan: make(chan bool, 1),
		SigTermChan: make(chan os.Signal, 1),
		stopChan:    make(chan bool, 1),
		SigTerm:     true,
	}
	ErrHandler.Ctx, ErrHandler.ctxCancel = context.WithCancel(ErrHandler.topCtx)

	ErrHandler.HandleFunc = ErrHandler.defaultHandleFunc
	return &ErrHandler
}

//New Here.
func New() *ErrHandler {
	ErrHandler := ErrHandler{
		topCtx:      context.Background(),
		errChan:     make(chan error, 1),
		errRespChan: make(chan bool, 1),
		SigTermChan: make(chan os.Signal, 1),
		stopChan:    make(chan bool, 1),
		SigTerm:     true,
	}
	ErrHandler.Ctx, ErrHandler.ctxCancel = context.WithCancel(ErrHandler.topCtx)

	ErrHandler.HandleFunc = ErrHandler.defaultHandleFunc
	return &ErrHandler
}

//Watch Here.
func (eh *ErrHandler) Watch() {
	var stop bool
	var interrupt bool
	var currentErr Error
	if eh.SigTerm {
		signal.Notify(eh.SigTermChan, syscall.SIGINT, syscall.SIGTERM)
	}
	go func() {
		defer func() {
			eh.vLog("[Final ErrHandler Check]", "interrupt:", interrupt, "stop:", stop)
			fmt.Println("[err'd] ERROR:", currentErr.Value)
			if interrupt {
				eh.Cleanup()
			}
		}()
		for {
			if !stop {
				select {
				case sig := <-eh.SigTermChan:
					eh.vLog("[ErrHandler] Caught signal", sig, ": terminating ...")
					interrupt = true
					sigErr := fmt.Errorf("Caught %v signal", sig)
					/*
						eh.handleSigTerm(sigErr)
					*/
					eh.ctxCancel()
					stop = true
					eh.running = false
					panic(sigErr)

				case <-eh.Ctx.Done():
					eh.vLog("Ctx Done recieved.")
				case s := <-eh.stopChan:
					eh.vLog("[ErrHandler] Shutdown request received:", s)
					if s {
						eh.vLog("[ErrHandler] Calling Ctx Cancel")
						eh.ctxCancel()
						stop = s
						eh.running = false
					}
				case err := <-eh.errChan:
					eh.vLog("[ErrHandler] Possible Error received.")
					eh.vLog("[ErrHandler] Assigning Possible Error as Current Error")
					currentErr.Value = err
					eh.vLog("[ErrHandler] Passing Possible Error to HandleFunc")
					if ok := eh.HandleFunc(err); !ok {
						eh.vLog("ERROR FOUND")
						eh.errRespChan <- false
					} else {
						eh.vLog("[ErrHandler] Error Nil")
						eh.errRespChan <- true
					}
				default:
					eh.running = true
				}

				if !eh.running {
					eh.vLog("[ErrHandler] Shutting Down")
					break
				}

			}
		}
		eh.vLog("[ErrHandler] Stopped")
	}()

	for {
		if eh.running {
			eh.vLog("[ErrHandler] Now running")
			break
		}
		eh.vLog("[ErrHandler] Not running Yet")
	}
	//return eh.running
}

//Stop Here.
func (eh *ErrHandler) Stop() {
	eh.vLog("[Stop()] Request Received")
	eh.vLog("[Stop()] Sending shutdown request to ErrorHandler")
	eh.stopChan <- true
}

//Running returns true if eh is running, false if not.
func (eh *ErrHandler) Running() bool {
	return eh.running
}

//Handle Here.
func (eh *ErrHandler) Handle(err error) {
	eh.vLog("[Handle()] Request Received")
	eh.vLog("[Handle()] Sending error to ErrorHandler")
	if ok := eh.sendError(err); !ok {
		eh.vLog("[Handle()] Error Found, Attempting Clean Exit")
		eh.Stop()
		panic(err)
	}
}

//HaltIf Halts (calling log.Fatalf()) immediately returing the error.
func (eh *ErrHandler) HaltIf(err error) {
	eh.vLog("[HaltIf()] Request Received")
	eh.vLog("[HaltIf()] Sending error to ErrorHandler")
	if ok := eh.sendError(err); !ok {
		eh.vLog("[HaltIf()] Error Found, Halting")
		log.Fatalf("Halting Error: %v\n", err)
	}
}

func (eh *ErrHandler) handleSigTerm(err error) {
	eh.vLog("[handleSigTerm()] Request Received")
	eh.vLog("[handleSigTerm()] Sending error to ErrorHandler")
	/*
		eh.Stop()
		panic(err)
	*/
	if ok := eh.sendError(err); !ok {
		eh.vLog("[handleSigTerm()] Error Found, Attempting Clean Exit")
		eh.Stop()
		panic(err)
	}
}

func (eh *ErrHandler) sendError(err error) bool {
	eh.vLog("[sendError()] Request Received")
	var resp bool
	eh.vLog("[sendError()] Sending Error")
	eh.errChan <- err
	eh.vLog("[sendError()] Waiting for Response")
	for {
		resp = <-eh.errRespChan
		break
	}
	eh.vLog("[sendError()] Response Passed:", resp)
	return resp
}

func (eh *ErrHandler) defaultHandleFunc(err error) bool {
	eh.vLog("[defaultHandleFunc()] Request Received")
	if err != nil {
		eh.vLog("[defaultHandleFunc()] Error !nil")
		eh.vLog("[defaultHandleFunc()] Error returned:", err)
		return false
	}
	return true
}

//Cleanup Here.
func (eh *ErrHandler) Cleanup() {
	eh.vLog("Cleanup Called")
	if r := recover(); r != nil {
		for eh.running {
			eh.Log("Performing Cleanup ... ")
		}
		eh.vLog("Recovered in cleanup:", r)
	}
	time.Sleep(time.Second * 3)
}

//VerboseLogging enables internal verbose logging of errd.
func (eh *ErrHandler) VerboseLogging() {
	eh.verbose = true
	eh.verboseLogger = log.New(os.Stdout, "[err'd] ", log.LstdFlags)
}

//Log if Verbose is set true.
func (eh *ErrHandler) vLog(a ...interface{}) {
	if eh.verbose {
		eh.verboseLogger.Println(a...)
	}
}

//EnableLogging enables verbose logging.
func (eh *ErrHandler) EnableLogging(prefix string) {
	eh.logging = true
	eh.Logger = log.New(os.Stdout, prefix, log.LstdFlags)
}

//Log if Verbose is set true.
func (eh *ErrHandler) Log(a ...interface{}) {
	if eh.logging {
		eh.Logger.Println(a...)
	}
}
