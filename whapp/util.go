package whapp

import "github.com/chromedp/cdproto/runtime"

func awaitPromise(params *runtime.EvaluateParams) *runtime.EvaluateParams {
	return params.WithAwaitPromise(true)
}
