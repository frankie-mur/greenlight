package data

import (
	"fmt"
	"strconv"
)

type Runtime int32

// usepointer receiver asgives us more flexibility
// means that our custom JSON encoding will work on Runtime values and pointers to Runtime values
func (r Runtime) MarshalJSON() ([]byte, error) {
	jsonValue := fmt.Sprintf("%d mins", r)
	//Use srtconv.Quote() to wrap in double quotes,
	// this is needed to be valid *JSON string*
	quotedJSONVal := strconv.Quote(jsonValue)

	return []byte(quotedJSONVal), nil
}
