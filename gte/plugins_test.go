package get

import (
	"fmt"
	"github.com/google/uuid"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"reflect"

	"testing"
)

type UserTest struct {
	Phone string `json:"phone,omitempty" gorm:"comment:电话"`
	Email string `json:"email,omitempty" gorm:"comment:邮箱"`
}

func (ut UserTest) Comment() string {
	return "用户"
}

func Connect(plugin gorm.Plugin) *gorm.DB {
	// 连接主数据库
	client, err := gorm.Open(mysql.Open("root:@tcp(127.0.0.1:3306)/basic_platform?charset=utf8mb4&parseTime=True&loc=Local"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
			TablePrefix:   "_res",
		},
	})
	if err != nil {
		panic(err)
	}

	client.Use(plugin)

	return client
}

func TestErrorPlugin_DuplicatedKey(t *testing.T) {
	plugin := NewGormErrorPlugin()

	db := Connect(plugin)
	db.AutoMigrate(UserTest{})
	db.Create(UserTest{Phone: "18286219254"})
	db.Create(UserTest{Phone: "18286219255"})
	db.Create(UserTest{Email: "1280291001@qq.com"})
	db.Create(UserTest{Email: "1280291002@qq.com"})

	tests := []struct {
		input UserTest
		want  string
	}{
		{
			input: UserTest{Phone: "18286219254", Email: uuid.NewString()},
			want:  "用户的电话已存在:18286219254",
		},
		{
			input: UserTest{Phone: "18286219255", Email: uuid.NewString()},
			want:  "用户的电话已存在:18286219255",
		},
		{
			input: UserTest{Phone: uuid.NewString(), Email: "1280291001@qq.com"},
			want:  "用户的邮箱已存在:1280291001@qq.com",
		},
		{
			input: UserTest{Phone: uuid.NewString(), Email: "1280291002@qq.com"},
			want:  "用户的邮箱已存在:1280291002@qq.com",
		},
	}

	for _, test := range tests {
		if err := db.Create(test.input).Error; err != nil {
			if !reflect.DeepEqual(err.Error(), test.want) {
				t.Errorf("转换失败%v:%v", test.input, err.Error())
			}
		}
	}
}

type UserTestRef struct {
	Phone string `json:"phone,omitempty" gorm:"comment:电话"`
	Email string `json:"email,omitempty" gorm:"comment:邮箱"`
	Other string `json:"other,omitempty" gorm:"comment:其他"`
}

func (ut UserTestRef) Comment() string {
	return "用户引用"
}

func TestErrorPlugin_ForeignKeyViolated(t *testing.T) {
	plugin := NewGormErrorPlugin()
	db := Connect(plugin)
	db.AutoMigrate(UserTestRef{})

	err := db.Create(UserTestRef{Email: "1280291001@qq.com", Phone: "18286219257", Other: uuid.NewString()})
	fmt.Println(err)
}

func TestErrorPlugin_Register(t *testing.T) {
	plugin := NewGlobalGormErrorPlugin()
	//db := Connect(plugin)
	plugin.Register(UserTestRef{})
	fmt.Println("==")
}
