package user

import (
	"embed"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/common"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/config"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/model"
	"github.com/TangSengDaoDao/TangSengDaoDaoServerLib/pkg/register"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

//go:embed sql
var sqlFS embed.FS

//go:embed swagger/api.yaml
var swaggerContent string

//go:embed swagger/friend.yaml
var friendSwaggerContent string

//go:embed txt/names.txt
var nameFS embed.FS
var Nicknames []string
var localRand *rand.Rand

func init() {

	// ====================== 注册用户模块 ======================
	register.AddModule(func(ctx interface{}) register.Module {
		x := ctx.(*config.Context)
		api := New(x)
		return register.Module{
			Name: "user",
			SetupAPI: func() register.APIRouter {
				return api
			},
			Swagger: swaggerContent,
			SQLDir:  register.NewSQLFS(sqlFS),
			IMDatasource: register.IMDatasource{
				SystemUIDs: func() ([]string, error) {
					users, err := api.userService.GetUsersWithCategories([]string{CategoryCustomerService, CategorySystem})
					if err != nil {
						return nil, err
					}
					uids := make([]string, 0, len(users))
					if len(users) > 0 {
						for _, user := range users {
							uids = append(uids, user.UID)
						}
					}
					return uids, nil
				},
			},
			BussDataSource: register.BussDataSource{
				ChannelGet: func(channelID string, channelType uint8, loginUID string) (*model.ChannelResp, error) {
					if channelType != common.ChannelTypePerson.Uint8() {
						return nil, register.ErrDatasourceNotProcess
					}
					userDetailResp, err := api.userService.GetUserDetail(channelID, loginUID)
					if err != nil {
						return nil, err
					}
					if userDetailResp == nil {
						api.Error("用户不存在！", zap.String("channel_id", channelID))
						return nil, errors.New("用户不存在！")
					}
					return newChannelRespWithUserDetailResp(userDetailResp), nil
				},
				GetDevice: func(ids []int64) ([]*model.DeviceResp, error) {
					list, err := api.deviceDB.queryDevicesWithIds(ids)
					if err != nil {
						return nil, err
					}
					if len(list) == 0 {
						return nil, nil
					}
					result := make([]*model.DeviceResp, 0, len(list))
					for _, device := range list {
						result = append(result, &model.DeviceResp{
							ID:          device.Id,
							UID:         device.UID,
							DeviceID:    device.DeviceID,
							DeviceName:  device.DeviceName,
							DeviceModel: device.DeviceModel,
						})
					}
					return result, nil
				},
			},
		}
	})

	// ====================== 注册好友模块 ======================
	register.AddModule(func(ctx interface{}) register.Module {
		api := NewFriend(ctx.(*config.Context))
		return register.Module{
			Name: "friend",
			SetupAPI: func() register.APIRouter {
				return api
			},
			Swagger: friendSwaggerContent,
			IMDatasource: register.IMDatasource{
				HasData: func(channelID string, channelType uint8) register.IMDatasourceType {
					if channelType == common.ChannelTypePerson.Uint8() {
						return register.IMDatasourceTypeWhitelist
					}
					return register.IMDatasourceTypeNone
				},
				Whitelist: func(channelID string, channelType uint8) ([]string, error) {
					friends, err := api.userService.GetFriends(channelID)
					if err != nil {
						return nil, err
					}
					firendUIDs := make([]string, 0, len(friends))
					if len(friends) > 0 {
						for _, friend := range friends {
							if friend.IsAlone == 0 {
								firendUIDs = append(firendUIDs, friend.UID)
							}
						}
					}
					return firendUIDs, nil
				},
			},
			BussDataSource: register.BussDataSource{
				GetFriends: func(uid string) ([]*model.FriendResp, error) {
					friends, err := api.userService.GetFriends(uid)
					if err != nil {
						return nil, err
					}
					list := make([]*model.FriendResp, 0, len(friends))
					for _, friend := range friends {
						list = append(list, &model.FriendResp{
							Remark:  friend.Remark,
							ToUID:   friend.UID,
							IsAlone: friend.IsAlone,
						})
					}
					return list, nil
				},
			},
		}
	})

	// ====================== 注册用户管理模块 ======================
	register.AddModule(func(ctx interface{}) register.Module {

		return register.Module{
			Name: "user_manager",
			SetupAPI: func() register.APIRouter {
				return NewManager(ctx.(*config.Context))
			},
		}
	})
	initNamesPlace()

}
func initNamesPlace() {
	const namesFilePath = "txt/names.txt"
	// 初始化随机数种子，确保每次启动程序生成的序列不同
	localRand = rand.New(rand.NewSource(time.Now().UnixNano()))
	data, err := nameFS.ReadFile(namesFilePath)
	if err != nil {
		panic("Failed to read embedded file: " + err.Error())
	}

	// b. 将字节数据转换为字符串
	content := string(data)

	// c. 清除首尾空白符（包括换行），然后按行分割
	// 这将把 "网名1\n网名2\n" 转换为 ["网名1", "网名2"]
	Nicknames = strings.Split(strings.TrimSpace(content), "\n")

	// 过滤掉可能存在的空行，确保切片只包含有效网名
	var cleanNames []string
	for _, name := range Nicknames {
		if name != "" {
			cleanNames = append(cleanNames, name)
		}
	}
	Nicknames = cleanNames
}

func newChannelRespWithUserDetailResp(user *UserDetailResp) *model.ChannelResp {

	resp := &model.ChannelResp{}
	resp.Channel.ChannelID = user.UID
	resp.Channel.ChannelType = uint8(common.ChannelTypePerson)
	resp.Name = user.Name
	resp.Username = user.Username
	resp.Logo = fmt.Sprintf("users/%s/avatar", user.UID)
	resp.Mute = user.Mute
	resp.Stick = user.Top
	resp.Receipt = user.Receipt
	resp.Robot = user.Robot
	resp.Online = user.Online
	resp.LastOffline = int64(user.LastOffline)
	resp.DeviceFlag = user.DeviceFlag
	resp.Category = user.Category
	resp.Follow = user.Follow
	resp.Remark = user.Remark
	resp.Status = user.Status
	resp.BeBlacklist = user.BeBlacklist
	resp.BeDeleted = user.BeDeleted
	resp.Flame = user.Flame
	resp.FlameSecond = user.FlameSecond
	extraMap := make(map[string]interface{})
	extraMap["sex"] = user.Sex
	extraMap["chat_pwd_on"] = user.ChatPwdOn
	extraMap["short_no"] = user.ShortNo
	extraMap["source_desc"] = user.SourceDesc
	extraMap["vercode"] = user.Vercode
	extraMap["screenshot"] = user.Screenshot
	extraMap["revoke_remind"] = user.RevokeRemind
	resp.Extra = extraMap

	return resp
}
