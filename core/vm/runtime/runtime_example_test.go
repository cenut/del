package runtime_test
import (
	"fmt"
	"github.com/DEL-ORG/del/common"
	"github.com/DEL-ORG/del/core/vm/runtime"
)
func ExampleExecute() {
	ret, _, err := runtime.Execute(common.Hex2Bytes("6060604052600a8060106000396000f360606040526008565b00"), nil, nil)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(ret)
}
