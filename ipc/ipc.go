package ipc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type IPC struct {
	BaseUrl string
}
type User struct {
	Id       uint   `json:"id"`
	FullName string `json:"full_name"`
}

func (ipc IPC) GetUser(userId uint) (User, int, error) {
	user := User{}
	res, err := http.Get(ipc.BaseUrl + fmt.Sprintf("/userinfo/?id=%v", userId))
	if err != nil {
		return user, 0, err
	}
	if err != nil {
		return user, 0, err
	}
	if res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusInternalServerError {
		return user, res.StatusCode, nil
	}
	if res.Body != nil {
		defer res.Body.Close()
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return user, res.StatusCode, err
	}
	if err := json.Unmarshal(body, &user); err != nil {
		return user, res.StatusCode, err
	}
	return user, res.StatusCode, nil
}
