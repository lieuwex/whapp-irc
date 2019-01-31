package whapp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/client"
	"github.com/chromedp/chromedp/runner"
)

// TODO: wrap unknown chromedp errors in an error type.

type internalUnit interface {
	Run(context.Context, chromedp.Action) error
	Shutdown(context.Context, ...client.Option) error
}

type poolUnit struct {
	res *chromedp.Res
}

func (u *poolUnit) Run(ctx context.Context, action chromedp.Action) error {
	return u.res.Run(ctx, action)
}
func (u *poolUnit) Shutdown(ctx context.Context, opts ...client.Option) error {
	cdp := u.res.CDP()
	if err := cdp.Shutdown(ctx); err != nil {
		return err
	}
	return u.res.Release()
}

// Instance is an instance to Whatsapp Web.
type Instance struct {
	LoginState LoginState

	unit     internalUnit
	cdp      *chromedp.CDP
	injected bool
}

func getOptions(headless bool) []runner.CommandLineOption {
	return []runner.CommandLineOption{
		runner.KillProcessGroup,
		runner.ForceKill,

		runner.Flag("headless", headless),
		runner.DisableGPU,
		runner.NoSandbox,

		runner.NoFirstRun,
		runner.NoDefaultBrowserCheck,

		runner.UserAgent(userAgent),
	}
}

// MakeInstance makes a new Instance.
func MakeInstance(
	ctx context.Context,
	headless bool,
	loggingLevel LoggingLevel,
) (*Instance, error) {
	options := chromedp.WithRunnerOptions(getOptions(headless)...)

	cdp, err := func() (*chromedp.CDP, error) {
		switch loggingLevel {
		case LogLevelVerbose:
			return chromedp.New(ctx, options, chromedp.WithLog(log.Printf))
		default:
			return chromedp.New(ctx, options)
		}
	}()
	if err != nil {
		return nil, err
	}

	return &Instance{
		LoginState: Loggedout,

		unit:     cdp,
		cdp:      cdp,
		injected: false,
	}, nil
}

// MakeInstanceWithPool makes a new Instance using the given pool.
func MakeInstanceWithPool(
	ctx context.Context,
	pool *chromedp.Pool,
	headless bool,
	loggingLevel LoggingLevel,
) (*Instance, error) {
	options := getOptions(headless)

	res, err := pool.Allocate(ctx, options...)
	if err != nil {
		return nil, err
	}

	return &Instance{
		LoginState: Loggedout,

		unit:     &poolUnit{res},
		cdp:      res.CDP(),
		injected: false,
	}, nil
}

// Navigate opens a tab with WhatsApp Web, without checking the login state.
func (wi *Instance) Navigate(ctx context.Context) error {
	return wi.cdp.Run(ctx, chromedp.Tasks{
		chromedp.Navigate(url),
		chromedp.WaitVisible("._2EZ_m, ._3ZW2E"),
	})
}

// Open opens a tab with WhatsApp Web and returns the current login state.
func (wi *Instance) Open(ctx context.Context) (LoginState, error) {
	var state LoginState
	var loggedIn bool

	if err := wi.cdp.Run(ctx, chromedp.Tasks{
		chromedp.Navigate(url),
		chromedp.Sleep(2 * time.Second), // HACK
		chromedp.WaitVisible("._2EZ_m, ._3ZW2E"),
		chromedp.Evaluate("document.getElementsByClassName('_3ZW2E').length > 0", &loggedIn),
	}); err != nil {
		return state, err
	}

	if loggedIn {
		state = Loggedin
	} else {
		state = Loggedout
	}

	wi.LoginState = state
	return state, nil
}

// GetLocalStorage retrieves and returns the localstorage of the current
// instance on the current tab.
// This method expects you to already have Whatsapp Web open.
func (wi *Instance) GetLocalStorage(ctx context.Context) (map[string]string, error) {
	var str string
	if err := wi.cdp.Run(
		ctx,
		chromedp.Evaluate("JSON.stringify(localStorage)", &str),
	); err != nil {
		return nil, err
	}

	var res map[string]string
	if err := json.Unmarshal([]byte(str), &res); err != nil {
		return nil, err
	}

	return res, nil
}

// SetLocalStorage adds all keys given by `localStorage` to the localStorage of
// the current instance.
func (wi *Instance) SetLocalStorage(ctx context.Context, localStorage map[string]string) error {
	var idc []byte
	var tasks chromedp.Tasks

	for key, val := range localStorage {
		str := fmt.Sprintf("localStorage.setItem(%s, %s)", strconv.Quote(key), strconv.Quote(val))
		tasks = append(tasks, chromedp.Evaluate(str, &idc))
	}

	return wi.cdp.Run(ctx, tasks)
}

// GetLoginCode retrieves the login code for the current instance.
// This can be used to generate a QR code which can be scanned using the
// Whatsapp mobile app.
func (wi *Instance) GetLoginCode(ctx context.Context) (string, error) {
	if wi.LoginState == Loggedin {
		return "", ErrLoggedIn
	}

	var code string
	var ok bool

	if err := wi.cdp.Run(ctx, chromedp.Tasks{
		chromedp.WaitVisible("[alt='Scan me!']"), // wait for QR
		chromedp.AttributeValue("._2EZ_m", "data-ref", &code, &ok),
	}); err != nil {
		return "", err
	}

	if !ok {
		return "", ErrCDPUnknown
	}

	return code, nil
}

// WaitLogin waits until the current instance has been done logging in. (the
// user scanned the QR code and is accepted)
func (wi *Instance) WaitLogin(ctx context.Context) error {
	if err := wi.cdp.Run(ctx, chromedp.WaitVisible("._3ZW2E")); err != nil {
		return err
	}
	wi.LoginState = Loggedin
	return nil
}

// GetMe returns the Me object for the current instance.
func (wi *Instance) GetMe(ctx context.Context) (Me, error) {
	var res Me

	if wi.LoginState != Loggedin {
		return res, ErrLoggedOut
	}

	if err := wi.inject(ctx); err != nil {
		return res, err
	}

	err := wi.cdp.Run(ctx, chromedp.Evaluate("Store.Conn.toJSON()", &res))
	if err != nil {
		return res, err
	}

	return res, nil
}

func (wi *Instance) getLoggedIn(ctx context.Context) (bool, error) {
	var res bool

	if err := wi.inject(ctx); err != nil {
		return res, err
	}

	action := chromedp.Evaluate("Store.Conn.clientToken != null", &res)
	return res, wi.cdp.Run(ctx, action)
}

// ListenLoggedIn listens for login state changes by polling it every
// `interval`.
func (wi *Instance) ListenLoggedIn(ctx context.Context, interval time.Duration) (<-chan bool, <-chan error) {
	errCh := make(chan error)
	resCh := make(chan bool)

	go func() {
		defer close(errCh)
		defer close(resCh)

		prev := false
		first := true

		for {
			select {
			case <-ctx.Done():
				return

			case <-time.After(interval):
				res, err := wi.getLoggedIn(ctx)
				if err != nil {
					errCh <- err
					return
				}

				if res != prev && !first {
					resCh <- res
				}

				prev = res
				first = false
			}
		}
	}()

	return resCh, errCh
}

func (wi *Instance) getNewMessages(ctx context.Context) ([]Message, error) {
	var res []Message

	if wi.LoginState != Loggedin {
		return res, ErrLoggedOut
	}

	if err := wi.inject(ctx); err != nil {
		return res, err
	}

	if err := wi.cdp.Run(
		ctx,
		chromedp.Evaluate("whappGo.getNewMessages()", &res),
	); err != nil {
		return res, err
	}

	sort.SliceStable(res, func(i, j int) bool {
		return res[i].Timestamp < res[j].Timestamp
	})

	return res, nil
}

// ListenForMessages listens for new messages by polling every `interval`.
func (wi *Instance) ListenForMessages(ctx context.Context, interval time.Duration) (<-chan Message, <-chan error) {
	errCh := make(chan error)
	messageCh := make(chan Message)

	go func() {
		defer close(errCh)
		defer close(messageCh)

		for {
			select {
			case <-ctx.Done():
				return

			case <-time.After(interval):
				res, err := wi.getNewMessages(ctx)
				if err != nil {
					errCh <- err
					return
				}

				for _, msg := range res {
					messageCh <- msg
				}
			}
		}
	}()

	return messageCh, errCh
}

// SendMessageToChatID sends the given `message` to the chat with the given
// `chatID`.
func (wi *Instance) SendMessageToChatID(ctx context.Context, chatID ID, message string) error {
	str := fmt.Sprintf(
		"whappGo.sendMessage(%s, %s)",
		strconv.Quote(chatID.String()),
		strconv.Quote(message),
	)
	return runLoggedinWithoutRes(ctx, wi, str, false)
}

// GetAllChats returns a slice containing all the chats the user has
// participated in.
func (wi *Instance) GetAllChats(ctx context.Context) ([]Chat, error) {
	var res []Chat

	if wi.LoginState != Loggedin {
		return res, ErrLoggedOut
	}

	if err := wi.inject(ctx); err != nil {
		return res, err
	}

	if err := wi.cdp.Run(
		ctx,
		chromedp.Evaluate("whappGo.getAllChats()", &res),
	); err != nil {
		return res, err
	}

	return res, nil
}

// GetPhoneActive returns Whether or not the user's phone is active.
func (wi *Instance) GetPhoneActive(ctx context.Context) (bool, error) {
	var res bool

	if wi.LoginState != Loggedin {
		return res, ErrLoggedOut
	}

	if err := wi.inject(ctx); err != nil {
		return res, err
	}

	if err := wi.cdp.Run(
		ctx,
		chromedp.Evaluate("whappGo.getPhoneActive()", &res),
	); err != nil {
		return res, err
	}

	return res, nil
}

// ListenForPhoneActiveChange listens for changes in the user's phone
// activity.
func (wi *Instance) ListenForPhoneActiveChange(ctx context.Context, interval time.Duration) (<-chan bool, <-chan error) {
	// REVIEW: does this actually work?

	// REVIEW: is this still correct when we get logged out?

	errCh := make(chan error)
	resCh := make(chan bool)

	go func() {
		defer close(errCh)
		defer close(resCh)

		prev := false
		first := true

		for {
			select {
			case <-ctx.Done():
				return

			case <-time.After(interval):
				res, err := wi.GetPhoneActive(ctx)
				if err != nil {
					errCh <- err
					return
				}

				if first || res != prev {
					prev = res
					first = false
					resCh <- res
				}
			}
		}
	}()

	return resCh, errCh
}

// Shutdown shuts down the current Instance.
func (wi *Instance) Shutdown(ctx context.Context) error {
	return wi.unit.Shutdown(ctx)
}
