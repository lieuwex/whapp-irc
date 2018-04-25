package whapp

import (
	"context"
	"io/ioutil"
	"net/http"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

func awaitPromise(params *runtime.EvaluateParams) *runtime.EvaluateParams {
	return params.WithAwaitPromise(true)
}

func downloadFile(url string) ([]byte, error) {
	res, err := http.Get(url)
	if err != nil {
		return []byte{}, err
	}
	defer res.Body.Close()

	return ioutil.ReadAll(res.Body)
}

func runLoggedinWithoutRes(ctx context.Context, wi *Instance, code string) error {
	if wi.LoginState != Loggedin {
		return ErrLoggedOut
	}

	if err := wi.inject(ctx); err != nil {
		return err
	}

	var idc []byte
	return wi.cdp.Run(ctx, chromedp.Evaluate(code, &idc, awaitPromise))
}
