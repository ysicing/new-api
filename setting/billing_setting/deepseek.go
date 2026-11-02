package billing_setting

const deepSeekPeak = `weekday("Asia/Shanghai") >= 1 && weekday("Asia/Shanghai") <= 5 && ((hour("Asia/Shanghai") >= 9 && hour("Asia/Shanghai") < 12) || (hour("Asia/Shanghai") >= 14 && hour("Asia/Shanghai") < 18))`

const deepSeekFlashExpr = deepSeekPeak + ` ? tier("peak", p * 0.3 + cr * 0.006 + c * 1.2) : tier("off_peak", p * 0.15 + cr * 0.003 + c * 0.6)`

var builtinBillingExpr = map[string]string{
	"deepseek-flash":               deepSeekFlashExpr,
	"deepseek-v4-flash":            deepSeekFlashExpr,
	"deepseek-v4-flash-vision-exp": deepSeekFlashExpr,
	"deepseek-v4-pro":              deepSeekPeak + ` ? tier("peak", p * 1.32 + cr * 0.044 + c * 3.96) : tier("off_peak", p * 0.66 + cr * 0.022 + c * 1.98)`,
}
