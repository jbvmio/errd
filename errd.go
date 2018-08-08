package errd

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"
)

//ErrHandler Here.
type ErrHandler struct {
	topCtx         context.Context
	Ctx            context.Context
	ctxCancel      context.CancelFunc
	errChan        chan error
	errRespChan    chan bool
	stopChan       chan bool
	HandleFunc     func(error) bool
	running        bool
	verbose        bool
	internal       bool
	Logger         *log.Logger
	verboseLogger  *log.Logger
	internalLogger *log.Logger
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
		stopChan:    make(chan bool, 1),
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
		stopChan:    make(chan bool, 1),
	}
	ErrHandler.Ctx, ErrHandler.ctxCancel = context.WithCancel(ErrHandler.topCtx)
	ErrHandler.enableLogging("")

	ErrHandler.HandleFunc = defaultHandleFunc
	return &ErrHandler
}

//Watch Here.
func (eh *ErrHandler) Watch() {
	var stop bool
	var interrupt bool
	var currentErr Error
	go func() {
		defer func() {
			eh.iLog("[Final ErrHandler Check]", "interrupt:", interrupt, "stop:", stop)
			eh.Log("ERROR:", currentErr.Value)
			if interrupt {
				eh.Cleanup()
			}
		}()
		for {
			if !stop {
				select {
				case <-eh.Ctx.Done():
					eh.iLog("Ctx Done recieved.")
				case s := <-eh.stopChan:
					eh.iLog("[ErrHandler] Shutdown request received:", s)
					if s {
						eh.iLog("[ErrHandler] Calling Ctx Cancel")
						eh.ctxCancel()
						stop = s
						eh.running = false
					}
				case err := <-eh.errChan:
					eh.iLog("[ErrHandler] Possible Error received.")
					eh.iLog("[ErrHandler] Assigning Possible Error as Current Error")
					currentErr.Value = err
					eh.iLog("[ErrHandler] Passing Possible Error to HandleFunc")
					if ok := eh.HandleFunc(err); !ok {
						eh.iLog("ERROR FOUND")
						eh.errRespChan <- false
					} else {
						eh.iLog("[ErrHandler] Error Nil")
						eh.errRespChan <- true
					}
				default:
					eh.running = true
				}

				if !eh.running {
					eh.iLog("[ErrHandler] Shutting Down")
					break
				}

			}
		}
		eh.iLog("[ErrHandler] Stopped")
	}()

	for {
		if eh.running {
			eh.iLog("[ErrHandler] Now running")
			break
		}
		eh.iLog("[ErrHandler] Not running Yet")
	}
	//return eh.running
}

//Stop Here.
func (eh *ErrHandler) Stop() {
	eh.iLog("[Stop()] Request Received")
	eh.iLog("[Stop()] Sending shutdown request to ErrorHandler")
	eh.stopChan <- true
}

//Running returns true if eh is running, false if not.
func (eh *ErrHandler) Running() bool {
	return eh.running
}

//Handle Here.
func (eh *ErrHandler) Handle(err error) {
	eh.iLog("[Handle()] Request Received")
	eh.iLog("[Handle()] Sending error to ErrorHandler")
	if ok := eh.sendError(err); !ok {
		eh.iLog("[Handle()] Error Found, Attempting Clean Exit")
		eh.Stop()
		panic(err)
	}
}

//HaltIf Halts (calling log.Fatalf()) immediately returing the error.
func (eh *ErrHandler) HaltIf(err error) {
	eh.iLog("[HaltIf()] Request Received")
	eh.iLog("[HaltIf()] Sending error to ErrorHandler")
	if ok := eh.sendError(err); !ok {
		eh.iLog("[HaltIf()] Error Found, Halting")
		log.Fatalf("Halting Error: %v\n", err)
	}
}

func (eh *ErrHandler) sendError(err error) bool {
	eh.iLog("[sendError()] Request Received")
	var resp bool
	eh.iLog("[sendError()] Sending Error")
	eh.errChan <- err
	eh.iLog("[sendError()] Waiting for Response")
	for {
		resp = <-eh.errRespChan
		break
	}
	eh.iLog("[sendError()] Response Passed:", resp)
	return resp
}

func (eh *ErrHandler) defaultHandleFunc(err error) bool {
	eh.iLog("[defaultHandleFunc()] Request Received")
	if err != nil {
		eh.iLog("[defaultHandleFunc()] Error !nil")
		eh.iLog("[defaultHandleFunc()] Error returned:", err)
		return false
	}
	return true
}

func defaultHandleFunc(err error) bool {
	if err != nil {
		return false
	}
	return true
}

//Cleanup Here.
func (eh *ErrHandler) Cleanup() {
	eh.iLog("Cleanup Called")
	if r := recover(); r != nil {
		for eh.running {
			eh.Log("Performing Cleanup ... ")
		}
		eh.iLog("Recovered in cleanup:", r)
	}
	time.Sleep(time.Second * 3)
}

//InternalLogging enables internal verbose logging of errd.
func (eh *ErrHandler) InternalLogging() {
	eh.internal = true
	eh.internalLogger = log.New(os.Stdout, "[err'd] ", log.LstdFlags)
}

//Log if Verbose is set true.
func (eh *ErrHandler) iLog(a ...interface{}) {
	if eh.internal {
		eh.internalLogger.Println(a...)
	}
}

//VerboseLogging enables verbose logging for troubleshooting and debugging purposes.
func (eh *ErrHandler) VerboseLogging() {
	eh.verbose = true
	eh.verboseLogger = log.New(os.Stdout, "[verbose] ", log.LstdFlags)
}

//SetVP sets the prefix for the verbose logger.
func (eh *ErrHandler) SetVP(prefix string) {
	eh.Logger.SetPrefix(prefix)
}

//Vog if Verbose is set true.
func (eh *ErrHandler) Vog(a ...interface{}) {
	if eh.verbose {
		eh.verboseLogger.Println(a...)
	}
}

//EnableLogging enables verbose logging.
func (eh *ErrHandler) enableLogging(prefix string) {
	//eh.logging = true
	eh.Logger = log.New(os.Stdout, prefix, log.LstdFlags)
}

//SetP sets the prefix for the logger.
func (eh *ErrHandler) SetP(prefix string) {
	eh.Logger.SetPrefix(prefix)
}

//Log if Verbose is set true.
func (eh *ErrHandler) Log(a ...interface{}) {
	eh.Logger.Println(a...)
}
