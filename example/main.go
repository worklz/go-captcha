package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"

	"github.com/worklz/go-captcha"
)

func main() {
	// 创建一个新的 Captcha 实例
	captchaInstance := captcha.NewCaptcha()

	// 启动一个简单的 HTTP 服务器来显示验证码图片
	http.HandleFunc("/captcha", func(w http.ResponseWriter, r *http.Request) {

		// 生成验证码图片和哈希值
		imageBase64, hash, err := captchaInstance.GenerateImageBase64()
		if err != nil {
			log.Fatalf("Failed to generate captcha: %v", err)
		}

		// 打印验证码图片的 Base64 编码
		fmt.Printf("Captcha Image Base64: %s\n", imageBase64)
		fmt.Printf("Captcha Hash: %s\n", hash)
		w.Header().Set("Content-Type", "image/png")
		imageData, _ := base64.StdEncoding.DecodeString(imageBase64)
		_, err = w.Write(imageData)
		if err != nil {
			http.Error(w, "Failed to write image", http.StatusInternalServerError)
		}
	})

	fmt.Println("Starting server at :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
