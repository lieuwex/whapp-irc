package whapp

import "github.com/chromedp/cdproto/runtime"

func AwaitPromise(params *runtime.EvaluateParams) *runtime.EvaluateParams {
	return params.WithAwaitPromise(true)
}
