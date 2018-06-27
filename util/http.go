package util

import (
	"fmt"
	"net/http"
	"strconv"
)

func GetSingleValueQueryParameter(req *http.Request, param string, allowed ...string) (string, bool, error) {
	values, found := GetMultipleValueQueryParameter(req, param)
	if len(values) > 1 {
		return "", found, fmt.Errorf("specified multiple %v query parameters in the URL", param)
	}
	if len(values) < 1 {
		return "", found, nil
	}

	v := values[0]
	if len(allowed) > 0 {
		for _, a := range allowed {
			if v == a {
				return v, found, nil
			}
		}

		return "", found, fmt.Errorf("'%s' is not a valid value for parameter '%s'", v, param)
	}

	return v, found, nil
}

func GetBoolQueryParameter(req *http.Request, param string, defaultVal bool) (bool, bool, error) {
	val, found, err := GetSingleValueQueryParameter(req, param)
	if !found || err != nil {
		return defaultVal, found, err
	}

	boolVal, err := strconv.ParseBool(val)
	if err != nil {
		return defaultVal, false, err
	}

	return boolVal, true, nil
}

func GetMultipleValueQueryParameter(req *http.Request, param string) ([]string, bool) {
	query := req.URL.Query()
	values, found := query[param]
	return values, found
}
