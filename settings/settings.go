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
package settings

import (
	"strings"

	"github.com/dvaumoron/puzzleweb"
	"github.com/dvaumoron/puzzleweb/common"
	"github.com/dvaumoron/puzzleweb/config"
	"github.com/dvaumoron/puzzleweb/session"
	"github.com/gin-gonic/gin"
)

type settingsWidget struct {
	editHandler gin.HandlerFunc
	saveHandler gin.HandlerFunc
}

func (w settingsWidget) LoadInto(router gin.IRouter) {
	router.GET("/edit", w.editHandler)
	router.POST("/save", w.saveHandler)
}

func AddSettingsPage(site *puzzleweb.Site, settingsConfig config.BasicConfig[SettingsManager], args ...string) {
	logger := settingsConfig.Logger
	settingsManager := settingsConfig.Service

	size := len(args)
	editTmpl := "settings/edit.html"
	if size != 0 && args[0] != "" {
		editTmpl = args[0]
	}
	if size > 1 {
		logger.Info("AddSettingsPage should be called with 1 or 2 arguments.")
	}

	p := puzzleweb.MakeHiddenPage("settings")
	p.Widget = settingsWidget{
		editHandler: puzzleweb.CreateTemplate(func(data gin.H, c *gin.Context) (string, string) {
			userId := session.GetUserId(logger, c)
			if userId == 0 {
				return "", common.DefaultErrorRedirect(common.UnknownUserKey)
			}

			data["Settings"] = settingsManager.Get(userId, c)
			return editTmpl, ""
		}),
		saveHandler: common.CreateRedirect(func(c *gin.Context) string {
			userId := session.GetUserId(logger, c)
			if userId == 0 {
				return common.DefaultErrorRedirect(common.UnknownUserKey)
			}

			var targetBuilder strings.Builder
			targetBuilder.WriteString(common.GetBaseUrl(1, c))
			targetBuilder.WriteString("edit")
			if err := settingsManager.Update(userId, c.PostFormMap("settings")); err != nil {
				common.WriteError(&targetBuilder, err.Error())
			}
			return targetBuilder.String()
		}),
	}

	site.AddPage(p)
}
