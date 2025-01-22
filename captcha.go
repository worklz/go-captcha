package captcha

import (
	"bytes"
	"crypto/md5"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

var (
	rng         *rand.Rand
	rngMutex    sync.Mutex
	rngInitOnce sync.Once
)

func init() {
	rngInitOnce.Do(func() {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	})
}

// 获取指定范围内的随机整数
func randomInt(min, max int) int {
	rngMutex.Lock()
	defer rngMutex.Unlock()
	if max < min {
		min, max = max, min
	}
	return rng.Intn(max-min+1) + min
}

// 嵌入字体资源
//
//go:embed fonts/*.ttf
var defaultEmbeddedFontsFS embed.FS

// 验证码存储接口
type StoreInterface interface {
	Set(hash, code string) error
	Get(hash string) (string, error)
}

// 验证码
type Captcha struct {
	// 验证码字符集合
	codeSet string
	// 验证码字体大小(px)
	fontSize int
	// 是否画混淆曲线
	useCurve bool
	// 是否添加杂点
	useNoise bool
	// 验证码位数
	length int
	// 背景颜色（红、绿、蓝）
	bg [3]int
	// 验证码图片高度
	imageH int
	// 验证码图片宽度
	imageW int
	// 验证码存储对象
	store StoreInterface
	// 字体
	trueTypeFontSlice []*truetype.Font
}

// 创建新的验证码对象
func NewCaptcha(store StoreInterface) *Captcha {
	// 加载字体数据
	fontNums := []string{"1", "2", "3", "5", "6"}
	trueTypeFontSlice := make([]*truetype.Font, len(fontNums))
	for i, num := range fontNums {
		fileName := fmt.Sprintf("fonts/%s.ttf", num)
		fontBytes, err := defaultEmbeddedFontsFS.ReadFile(fileName)
		if err != nil {
			panic(fmt.Sprintf("Font file %s read exception!%v", fileName, err))
		}
		trueTypeFont, err := freetype.ParseFont(fontBytes)
		if err != nil {
			panic(fmt.Sprintf("Font file %s parse exception!%v", fileName, err))
		}
		if trueTypeFont == nil {
			panic(fmt.Sprintf("Font file %s parse exception2!", fileName))
		}
		trueTypeFontSlice[i] = trueTypeFont
	}

	return &Captcha{
		codeSet:           "2345678abcdefhijkmnpqrstuvwxyzABCDEFGHJKLMNPQRTUVWXY",
		fontSize:          29,
		useCurve:          true,
		useNoise:          true,
		length:            4,
		bg:                [3]int{243, 251, 254},
		store:             store,
		trueTypeFontSlice: trueTypeFontSlice,
	}
}

// 校验
func (c *Captcha) Check(hash, code string) (res bool, err error) {
	rawCode, err := c.store.Get(hash)
	if err != nil {
		return
	}
	return rawCode != "" && rawCode == code, nil
}

// 生成验证码
// 返回：hash值、验证码图片base64
func (c *Captcha) Generate() (hash string, imgBase64 string, err error) {
	// 获取随机字体
	if len(c.trueTypeFontSlice) == 0 {
		err = errors.New("no fonts available")
		return
	}
	fontIndex := randomInt(0, len(c.trueTypeFontSlice)-1)
	ft := c.trueTypeFontSlice[fontIndex]
	if ft == nil {
		err = fmt.Errorf("font at index %d is nil", fontIndex)
		return
	}
	// 生成code码
	hash, code := c.generateCode()
	// 计算图片宽高
	c.imageW = int(float64(c.length)*float64(c.fontSize)*1.5) + c.length*c.fontSize/2
	c.imageH = int(float64(c.fontSize) * 2.5)

	img := image.NewRGBA(image.Rect(0, 0, c.imageW, c.imageH))
	bgColor := color.RGBA{uint8(c.bg[0]), uint8(c.bg[1]), uint8(c.bg[2]), 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)

	face := truetype.NewFace(ft, &truetype.Options{
		Size: float64(c.fontSize),
	})
	// 验证码字体随机颜色
	color := color.RGBA{uint8(randomInt(1, 150)), uint8(randomInt(1, 150)), uint8(randomInt(1, 150)), 255}
	// 绘制杂点
	if c.useNoise {
		c.writeNoise(img, ft)
	}
	// 绘制曲线
	if c.useCurve {
		c.writeCurve(img, color)
		c.writeCurve(img, color)
	}
	// 绘制验证码
	text := strings.Split(code, "")
	for index, char := range text {
		x := int(float64(c.fontSize)*float64(index+1)*1.5) + randomInt(1, 10)
		y := c.fontSize + randomInt(10, 20)
		angle := float64(randomInt(-40, 40)) * (3.1415926 / 180.0)

		c.drawText(img, face, color, x, y, angle, char)
	}

	var buf bytes.Buffer
	err = png.Encode(&buf, img)
	if err != nil {
		return
	}
	// 保存验证码
	err = c.store.Set(hash, code)
	if err != nil {
		return
	}

	imgBase64 = base64.StdEncoding.EncodeToString(buf.Bytes())
	return
}

// 获取随机字体
func (c *Captcha) getRandomFont() (*truetype.Font, error) {
	files, err := filepath.Glob("./fonts/*.ttf")
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no font files found in")
	}
	fontPath := files[randomInt(0, len(files)-1)]
	fontBytes, err := os.ReadFile(fontPath)
	if err != nil {
		return nil, fmt.Errorf("error reading font file: %v", err)
	}
	ft, err := truetype.Parse(fontBytes)
	if err != nil {
		return nil, fmt.Errorf("error parsing font: %v [fontPath]=%s", err, fontPath)
	}
	return ft, nil
}

// 绘制曲线
func (c *Captcha) writeCurve(img *image.RGBA, color color.Color) {
	amplitude := float64(randomInt(1, c.imageH/2))                                 // 振幅
	phaseShift := float64(randomInt(c.imageH/2, c.imageH/4))                       //	频率偏移
	frequency := 2 * math.Pi / float64(randomInt(1, c.imageW*2-c.imageH)+c.imageH) // 周期
	startX := 0
	endX := c.imageW

	for px := startX; px <= endX; px++ {
		py := int(amplitude*math.Sin(frequency*float64(px)+phaseShift) + float64(c.imageH)/2)
		i := c.fontSize / 5
		for i > 0 {
			if px+i < c.imageW && py+i < c.imageH {
				img.Set(px+i, py+i, color)
			}
			i--
		}
	}
}

// 绘制杂点
func (c *Captcha) writeNoise(img *image.RGBA, ft *truetype.Font) {
	for i := 0; i < 10; i++ {
		noiseColor := color.RGBA{uint8(randomInt(150, 225)), uint8(randomInt(150, 225)), uint8(randomInt(150, 225)), 255}
		for j := 0; j < 5; j++ {
			c.drawStringOnImage(img, ft, string(c.codeSet[randomInt(0, len(c.codeSet)-1)]), 18, randomInt(-10, c.imageW), randomInt(-10, c.imageH), noiseColor)
		}
	}
}

// 绘制字符串在图片上
func (c *Captcha) drawStringOnImage(img *image.RGBA, ft *truetype.Font, text string, fontSize float64, x, y int, textColor color.Color) error {
	ctx := freetype.NewContext()
	ctx.SetDPI(72)
	ctx.SetFont(ft)
	ctx.SetFontSize(fontSize)
	ctx.SetClip(img.Bounds())
	ctx.SetDst(img)
	ctx.SetSrc(image.NewUniform(textColor))

	pt := fixed.P(x, y+int(ctx.PointToFixed(fontSize)>>6))

	_, err := ctx.DrawString(text, pt)
	if err != nil {
		return fmt.Errorf("error drawing string: %w", err)
	}

	return nil
}

// 绘制文字
func (c *Captcha) drawText(img *image.RGBA, face font.Face, color color.Color, x, y int, angle float64, text string) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color),
		Face: face,
	}
	d.Dot = freetype.Pt(x, y)
	d.DrawString(text)
}

// 生成验证码
func (c *Captcha) generateCode() (string, string) {
	characters := strings.Split(c.codeSet, "")
	code := ""
	for i := 0; i < c.length; i++ {
		code += characters[randomInt(0, len(characters)-1)]
	}

	hash := fmt.Sprintf("%x", md5.Sum([]byte(strconv.FormatInt(time.Now().UnixNano(), 10)+code)))
	return hash, code
}
