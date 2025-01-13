package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"

	"github.com/worklz/go-captcha"
)

// 创建验证码存储结构体，实现存储接口（captcha.go下的StoreInterface）
type Store struct {
}

// 实现将哈希值和验证码存储到数据库或其他存储中
func (s *Store) Set(hash, code string) error {
	return nil
}

// 实现从数据库或其他存储中获取哈希值对应的验证码
func (s *Store) Get(hash string) (string, error) {
	return "", nil
}

func main() {
	// 创建一个新的 Captcha 实例
	captchaInstance := captcha.NewCaptcha(&Store{})

	// 验证码图片
	// 浏览器访问：http://localhost:8080/captcha
	http.HandleFunc("/captcha", func(w http.ResponseWriter, r *http.Request) {
		// 生成验证码哈希值和图片base64编码
		hash, imgBase64, err := captchaInstance.Generate()
		if err != nil {
			log.Fatalf("Failed to generate captcha: %v", err)
		}
		fmt.Printf("Captcha Hash: %s\n", hash)
		w.Header().Set("Content-Type", "image/png")
		imageData, _ := base64.StdEncoding.DecodeString(imgBase64)
		_, err = w.Write(imageData)
		if err != nil {
			http.Error(w, "Failed to write image", http.StatusInternalServerError)
		}
	})

	// 验证验证码
	// 浏览器访问：http://localhost:8080/verify?hash=xxxx&code=xxxx
	http.HandleFunc("/verify", func(w http.ResponseWriter, r *http.Request) {
		hash := r.URL.Query().Get("hash")
		code := r.URL.Query().Get("code")
		res, err := captchaInstance.Check(hash, code)
		if err != nil {
			http.Error(w, "Failed to check captcha", http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "%v", res)
	})

	fmt.Println("Starting server at :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
