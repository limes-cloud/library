### Gorm Transform Error
#### 背景解读
假设我有一个表
``` 
create table user(
    id int not null primary_key auto_increment,
    phone char(11) not null comment '手机号',
    email varchar(64) binary not null comment '邮箱',
    id_card char(18) not null comment '身份证',
    unique(phone),
    unique(email),
    unique(id_card)
)engine innodb  comment '用户';
```
其中手机号，邮箱，身份证 都是唯一索引，前端用户会输入这些信息然后插入到表中。

在gorm中，我们通常会进行查询判断，比如下
``` 
type User{
    ...
}

// 参数 in.Phone 
if err = db.First(&User{},"phone=?",in.Phone);err==nil{
    return errors.New("用户手机号已存在")
}

// 参数 in.Email
if err = db.First(&User{},"email=?",in.Email);err==nil{
    return errors.New("邮箱已存在")
}

// 参数 in.IdCard
if err = db.First(&User{},"id_card=?",in.IdCard);err==nil{
    return errors.New("身份证已存在")
}

return db.Create(u).Error
```

这里有人或许要问了，我可以用or查询不就好了么，这样只用写一个查询就好了。的却是的，但是你这样只能检测出来数据是否重复，但是具体是哪一项重复，还是得写额外的代码进行判断，反而显得麻烦了。而且or是不会走索引的，会进行全表扫描，会增加数据库的压力。


#### 使用指南
##### 方法一 通过扫描数据表进行初始化
使用扫描数据表初始化需要注意数据表定义的时候确保已经定义了 comment
WithEnableLoad() 
``` 
// 连接主数据库
client, _ := gorm.Open(mysql.Open("root:@tcp(127.0.0.1:3306)/basic_platform?charset=utf8mb4&parseTime=True&loc=Local"), &gorm.Config{
	NamingStrategy: schema.NamingStrategy{
		SingularTable: true,
		TablePrefix:   "_res",
	},
})

plugin := NewGormErrorPlugin(WithEnableLoad())
client.Use(plugin)

// 直接插入，会自动转换error
client.Create(&u).Error

```
#### 方法二 通过手动注册初始化

``` 
// 连接主数据库
client, _ := gorm.Open(mysql.Open("root:@tcp(127.0.0.1:3306)/basic_platform?charset=utf8mb4&parseTime=True&loc=Local"), &gorm.Config{
	NamingStrategy: schema.NamingStrategy{
		SingularTable: true,
		TablePrefix:   "_res",
	},
})

type User struct {
	Phone string `json:"phone,omitempty" gorm:"comment:电话"` // 或者 comment:"电话"
	Email string `json:"email,omitempty" gorm:"comment:邮箱"` // 或者 comment:"电话"
}

func (ut UserTest) Comment() string {
	return "用户"
}

plugin := NewGlobalGormErrorPlugin(WithGorm(client))
plugin.Register(UserTestRef{})
plugin.Register(UserTest{})
```
