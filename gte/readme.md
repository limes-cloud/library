### Gorm Transform Error
##### 背景解读
在我们开发过程中，会遇到很多错误

##### 

``` 
type UserTestRef struct {
	Phone string `json:"phone,omitempty" gorm:"comment:电话"`
	Email string `json:"email,omitempty" gorm:"comment:邮箱"`
	Other string `json:"other,omitempty" gorm:"comment:其他"`
}

func (ut UserTestRef) Comment() string {
	return "用户引用"
}

type UserTest struct {
	Phone string `json:"phone,omitempty" gorm:"comment:电话"`
	Email string `json:"email,omitempty" gorm:"comment:邮箱"`
}

func (ut UserTest) Comment() string {
	return "用户"
}

func Connect() *gorm.DB {
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
	return client
}

func main(){
    client := Connect()
	plugin := NewGlobalGormErrorPlugin(WithGorm(client))
	
	plugin.Register(UserTestRef{})
	plugin.Register(UserTest{})
}
```