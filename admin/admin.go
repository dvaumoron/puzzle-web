/*
 *
 * Copyright 2022 puzzleweb authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */
package admin

import (
	"errors"
	"sort"
	"strings"

	pb "github.com/dvaumoron/puzzlerightservice"
	"github.com/dvaumoron/puzzleweb"
	"github.com/dvaumoron/puzzleweb/admin/client"
	"github.com/dvaumoron/puzzleweb/common"
	"github.com/dvaumoron/puzzleweb/locale"
	"github.com/dvaumoron/puzzleweb/log"
	loginclient "github.com/dvaumoron/puzzleweb/login/client"
	profileclient "github.com/dvaumoron/puzzleweb/profile/client"
	"github.com/dvaumoron/puzzleweb/session"
	"github.com/gin-gonic/gin"
)

const roleNameName = "RoleName"
const groupName = "Group"
const groupsName = "Groups"
const viewAdminName = "ViewAdmin"

const (
	accessKey = "AccessLabel"
	createKey = "CreateLabel"
	updateKey = "UpdateLabel"
	deleteKey = "DeleteLabel"
)

var errBadName = errors.New("ErrorBadRoleName")

var actionToKey = [4]string{accessKey, createKey, updateKey, deleteKey}

type GroupDisplay struct {
	Id           uint64
	Name         string
	DisplayName  string
	Roles        []RoleDisplay
	AddableRoles []RoleDisplay
}

type RoleDisplay struct {
	Name    string
	Actions []string
}

func MakeRoleDisplay(role client.Role, c *gin.Context) RoleDisplay {
	return RoleDisplay{Name: role.Name, Actions: displayActions(role.Actions, c)}
}

type sortableGroups []*GroupDisplay

func (s sortableGroups) Len() int {
	return len(s)
}

func (s sortableGroups) Less(i, j int) bool {
	return s[i].Id < s[j].Id
}

func (s sortableGroups) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

type sortableRoles []RoleDisplay

func (s sortableRoles) Len() int {
	return len(s)
}

func (s sortableRoles) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

func (s sortableRoles) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

type adminWidget struct {
	displayHandler  gin.HandlerFunc
	listUserHandler gin.HandlerFunc
	viewUserHandler gin.HandlerFunc
	editUserHandler gin.HandlerFunc
	listRoleHandler gin.HandlerFunc
	editRoleHandler gin.HandlerFunc
}

var saveUserHandler = common.CreateRedirect(func(c *gin.Context) string {
	userId := common.GetRequestedUserId(c)
	err := common.ErrTechnical
	if userId != 0 {
		rolesStr := c.PostFormArray("roles")
		roles := make([]client.Role, 0, len(rolesStr))
		for _, roleStr := range rolesStr {
			splitted := strings.Split(roleStr, "/")
			if len(splitted) > 1 {
				roles = append(roles, client.Role{
					Name: splitted[0], Group: splitted[1],
				})
			}
		}
		err = client.UpdateUser(session.GetUserId(c), userId, roles)
	}

	targetBuilder := userListUrlBuilder(c)
	if err != nil {
		common.WriteError(targetBuilder, err.Error())
	}
	return targetBuilder.String()
})

var deleteUserHandler = common.CreateRedirect(func(c *gin.Context) string {
	userId := common.GetRequestedUserId(c)
	err := common.ErrTechnical
	if userId != 0 {
		// an empty slice delete the user right
		// only the first service call do a right check
		err = client.UpdateUser(session.GetUserId(c), userId, []client.Role{})
		if err == nil {
			err = profileclient.Delete(userId)
			if err == nil {
				err = loginclient.DeleteUser(userId)
			}
		}
	}

	targetBuilder := userListUrlBuilder(c)
	if err != nil {
		common.WriteError(targetBuilder, err.Error())
	}
	return targetBuilder.String()
})

var saveRoleHandler = common.CreateRedirect(func(c *gin.Context) string {
	roleName := c.PostForm(roleNameName)
	err := errBadName
	if roleName != "new" {
		group := c.PostForm(groupName)
		actions := make([]pb.RightAction, 0, 4)
		for _, actionStr := range c.PostFormArray("actions") {
			var action pb.RightAction
			switch actionStr {
			case "access":
				action = client.ActionAccess
			case "create":
				action = client.ActionCreate
			case "update":
				action = client.ActionUpdate
			case "delete":
				action = client.ActionDelete
			}
			actions = append(actions, action)
		}
		err = client.UpdateRole(session.GetUserId(c), client.Role{Name: roleName, Group: group, Actions: actions})
	}

	var targetBuilder strings.Builder
	targetBuilder.WriteString(common.GetBaseUrl(1, c))
	targetBuilder.WriteString("list")
	if err != nil {
		common.WriteError(&targetBuilder, err.Error())
	}
	return targetBuilder.String()
})

func (w *adminWidget) LoadInto(router gin.IRouter) {
	router.GET("/", w.displayHandler)
	router.GET("/user/list", w.listUserHandler)
	router.GET("/user/view/:UserId", w.viewUserHandler)
	router.GET("/user/edit/:UserId", w.editUserHandler)
	router.POST("/user/save/:UserId", saveUserHandler)
	router.GET("/user/delete/:UserId", deleteUserHandler)
	router.GET("/role/list", w.listRoleHandler)
	router.GET("/role/edit/:RoleName/:Group", w.editRoleHandler)
	router.POST("/role/save", saveRoleHandler)
}

func adminData(data gin.H, c *gin.Context) {
	data[viewAdminName] = client.AuthQuery(session.GetUserId(c), client.AdminGroupId, client.ActionAccess) == nil
}

func AddAdminPage(site *puzzleweb.Site, args ...string) {
	indexTmpl := "admin/index.html"
	listUserTmpl := "admin/user/list.html"
	viewUserTmpl := "admin/user/view.html"
	editUserTmpl := "admin/user/edit.html"
	listRoleTmpl := "admin/role/list.html"
	editRoleTmpl := "admin/role/edit.html"
	switch len(args) {
	default:
		log.Logger.Info("AddAdminPage should be called with 1 to 7 arguments.")
		fallthrough
	case 6:
		if args[5] != "" {
			editRoleTmpl = args[5]
		}
		fallthrough
	case 5:
		if args[4] != "" {
			listRoleTmpl = args[4]
		}
		fallthrough
	case 4:
		if args[3] != "" {
			editUserTmpl = args[3]
		}
		fallthrough
	case 3:
		if args[2] != "" {
			viewUserTmpl = args[2]
		}
		fallthrough
	case 2:
		if args[1] != "" {
			listUserTmpl = args[1]
		}
	case 1:
		if args[0] != "" {
			indexTmpl = args[0]
		}
		fallthrough
	case 0:
	}

	p := puzzleweb.NewHiddenPage("admin")
	p.Widget = &adminWidget{
		displayHandler: puzzleweb.CreateTemplate(func(data gin.H, c *gin.Context) (string, string) {
			viewAdmin, _ := data[viewAdminName].(bool)
			if !viewAdmin {
				return "", common.DefaultErrorRedirect(common.ErrNotAuthorized.Error())
			}
			return indexTmpl, ""
		}),
		listUserHandler: puzzleweb.CreateTemplate(func(data gin.H, c *gin.Context) (string, string) {
			viewAdmin, _ := data[viewAdminName].(bool)
			if !viewAdmin {
				return "", common.DefaultErrorRedirect(common.ErrNotAuthorized.Error())
			}

			pageNumber, start, end, filter := common.GetPagination(c)

			total, users, err := loginclient.ListUsers(start, end, filter)
			if err != nil {
				return "", common.DefaultErrorRedirect(err.Error())
			}

			common.InitPagination(data, filter, pageNumber, end, total)
			data["Users"] = users
			data[common.BaseUrlName] = common.GetBaseUrl(1, c)
			common.InitNoELementMsg(data, len(users), c)
			return listUserTmpl, ""
		}),
		viewUserHandler: puzzleweb.CreateTemplate(func(data gin.H, c *gin.Context) (string, string) {
			adminId := session.GetUserId(c)
			userId := common.GetRequestedUserId(c)
			if userId == 0 {
				return "", common.DefaultErrorRedirect(common.ErrTechnical.Error())
			}

			roles, err := client.GetUserRoles(adminId, userId)
			if err != nil {
				return "", common.DefaultErrorRedirect(err.Error())
			}

			users, err := loginclient.GetUsers([]uint64{userId})
			if err != nil {
				return "", common.DefaultErrorRedirect(err.Error())
			}

			updateRight := client.AuthQuery(adminId, client.AdminGroupId, client.ActionUpdate) == nil

			user := users[userId]
			data[common.BaseUrlName] = common.GetBaseUrl(2, c)
			data[common.UserIdName] = userId
			data[common.UserLoginName] = user.Login
			data[common.RegistredAtName] = user.RegistredAt
			data[common.AllowedToUpdateName] = updateRight
			data[groupsName] = DisplayGroups(roles, c)
			return viewUserTmpl, ""
		}),
		editUserHandler: puzzleweb.CreateTemplate(func(data gin.H, c *gin.Context) (string, string) {
			adminId := session.GetUserId(c)
			userId := common.GetRequestedUserId(c)
			if userId == 0 {
				return "", common.DefaultErrorRedirect(common.ErrTechnical.Error())
			}

			allRoles, err := client.GetAllRoles(adminId)
			if err != nil {
				return "", common.DefaultErrorRedirect(err.Error())
			}

			userRoles, err := client.GetUserRoles(adminId, userId)
			if err != nil {
				return "", common.DefaultErrorRedirect(err.Error())
			}

			userIdToLogin, err := loginclient.GetUsers([]uint64{userId})
			if err != nil {
				return "", common.DefaultErrorRedirect(err.Error())
			}

			data[common.BaseUrlName] = common.GetBaseUrl(2, c)
			data[common.UserIdName] = userId
			data[common.UserLoginName] = userIdToLogin[userId].Login
			data[groupsName] = displayEditGroups(userRoles, allRoles, c)
			return editUserTmpl, ""
		}),
		listRoleHandler: puzzleweb.CreateTemplate(func(data gin.H, c *gin.Context) (string, string) {
			allRoles, err := client.GetAllRoles(session.GetUserId(c))
			if err != nil {
				return "", common.DefaultErrorRedirect(err.Error())
			}

			data[common.BaseUrlName] = common.GetBaseUrl(1, c)
			data[groupsName] = DisplayGroups(allRoles, c)
			return listRoleTmpl, ""
		}),
		editRoleHandler: puzzleweb.CreateTemplate(func(data gin.H, c *gin.Context) (string, string) {
			roleName := c.PostForm(roleNameName)
			group := c.PostForm(groupName)

			data[common.BaseUrlName] = common.GetBaseUrl(1, c)
			data[roleNameName] = roleName
			data[groupName] = group

			if roleName != "new" {
				actions, err := client.GetActions(session.GetUserId(c), roleName, group)
				if err != nil {
					return "", common.DefaultErrorRedirect(err.Error())
				}

				actionSet := common.MakeSet(actions)
				setActionChecked(data, actionSet, client.ActionAccess, "Access")
				setActionChecked(data, actionSet, client.ActionCreate, "Create")
				setActionChecked(data, actionSet, client.ActionUpdate, "Update")
				setActionChecked(data, actionSet, client.ActionDelete, "Delete")
			}

			return editRoleTmpl, ""
		}),
	}

	site.AddDefaultData(adminData)

	site.AddPage(p)
}

func DisplayGroups(roles []client.Role, c *gin.Context) []*GroupDisplay {
	nameToGroup := map[string]*GroupDisplay{}
	populateGroup(nameToGroup, roles, c, rolesAppender)
	return sortGroups(nameToGroup)
}

func populateGroup(nameToGroup map[string]*GroupDisplay, roles []client.Role, c *gin.Context, appender func(*GroupDisplay, client.Role, *gin.Context)) {
	for _, role := range roles {
		groupName := role.Group
		group := nameToGroup[groupName]
		if group == nil {
			group = &GroupDisplay{
				Id: client.GetGroupId(groupName), Name: groupName,
				DisplayName: locale.GetText("GroupLabel"+locale.CamelCase(groupName), c),
			}
			nameToGroup[groupName] = group
		}
		appender(group, role, c)
	}
}

func rolesAppender(group *GroupDisplay, role client.Role, c *gin.Context) {
	group.Roles = append(group.Roles, MakeRoleDisplay(role, c))
}

// convert a RightAction slice in a displayable string slice,
// always in the same order : access, create, update, delete
func displayActions(actions []pb.RightAction, c *gin.Context) []string {
	actionSet := common.MakeSet(actions)
	res := make([]string, len(actions))
	if actionSet.Contains(client.ActionAccess) {
		res = append(res, locale.GetText(accessKey, c))
	}
	if actionSet.Contains(client.ActionCreate) {
		res = append(res, locale.GetText(createKey, c))
	}
	if actionSet.Contains(client.ActionUpdate) {
		res = append(res, locale.GetText(updateKey, c))
	}
	if actionSet.Contains(client.ActionDelete) {
		res = append(res, locale.GetText(deleteKey, c))
	}
	return res
}

func sortGroups(nameToGroup map[string]*GroupDisplay) []*GroupDisplay {
	groupRoles := common.MapToValueSlice(nameToGroup)
	sort.Sort(sortableGroups(groupRoles))
	for _, group := range groupRoles {
		sort.Sort(sortableRoles(group.Roles))
		sort.Sort(sortableRoles(group.AddableRoles))
	}
	return groupRoles
}

func displayEditGroups(userRoles []client.Role, allRoles []client.Role, c *gin.Context) []*GroupDisplay {
	nameToGroup := map[string]*GroupDisplay{}
	populateGroup(nameToGroup, userRoles, c, rolesAppender)
	populateGroup(nameToGroup, allRoles, c, addableRolesAppender)
	return sortGroups(nameToGroup)
}

func addableRolesAppender(group *GroupDisplay, role client.Role, c *gin.Context) {
	group.AddableRoles = append(group.AddableRoles, MakeRoleDisplay(role, c))
}

func setActionChecked(data gin.H, actionSet common.Set[pb.RightAction], toTest pb.RightAction, name string) {
	if actionSet.Contains(toTest) {
		data[name] = true
	}
}

func userListUrlBuilder(c *gin.Context) *strings.Builder {
	targetBuilder := new(strings.Builder)
	// no need to erase and rewrite "user/"
	targetBuilder.WriteString(common.GetBaseUrl(2, c))
	targetBuilder.WriteString("list")
	return targetBuilder
}
