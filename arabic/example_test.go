package arabic_test

import (
	"fmt"

	"github.com/MohamedElashri/agrep/arabic"
)

func ExampleNormalize() {
	voweled := "مَدْرَسَةٌ" // مَدْرَسَةٌ
	unvoweled := "مدرسه"    // مدرسه

	fmt.Println(arabic.Normalize(voweled) == arabic.Normalize(unvoweled))
	fmt.Println(arabic.Normalize(voweled))
	// Output:
	// true
	// مدرسه
}
