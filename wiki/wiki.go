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
package wiki

import (
	"fmt"
	"strings"

	"github.com/dvaumoron/puzzleweb"
	"github.com/dvaumoron/puzzleweb/common"
	"github.com/dvaumoron/puzzleweb/config"
	"github.com/dvaumoron/puzzleweb/locale"
	"github.com/gin-gonic/gin"
)

const versionName = "version"
const versionsName = "Versions"
const viewMode = "/view/"
const listMode = "/list/"
const titleName = "title"
const wikiTitleName = "WikiTitle"
const wikiVersionName = "WikiVersion"
const wikiContentName = "WikiContent"

type wikiWidget struct {
	defaultHandler gin.HandlerFunc
	viewHandler    gin.HandlerFunc
	editHandler    gin.HandlerFunc
	saveHandler    gin.HandlerFunc
	listHandler    gin.HandlerFunc
	deleteHandler  gin.HandlerFunc
}

func (w wikiWidget) LoadInto(router gin.IRouter) {
	router.GET("/", w.defaultHandler)
	router.GET("/:lang/view/:title", w.viewHandler)
	router.GET("/:lang/edit/:title", w.editHandler)
	router.POST("/:lang/save/:title", w.saveHandler)
	router.GET("/:lang/list/:title", w.listHandler)
	router.GET("/:lang/delete/:title", w.deleteHandler)
}

func MakeWikiPage(wikiName string, wikiConfig config.WikiConfig) puzzleweb.Page {
	logger := wikiConfig.Logger
	wikiService := wikiConfig.Service
	markdownService := wikiConfig.MarkdownService

	defaultPage := "Welcome"
	viewTmpl := "wiki/view.html"
	editTmpl := "wiki/edit.html"
	listTmpl := "wiki/list.html"
	switch args := wikiConfig.Args; len(args) {
	default:
		logger.Info("MakeWikiPage should be called with 0 to 4 optional arguments.")
		fallthrough
	case 4:
		if args[3] != "" {
			listTmpl = args[3]
		}
		fallthrough
	case 3:
		if args[2] != "" {
			editTmpl = args[2]
		}
		fallthrough
	case 2:
		if args[1] != "" {
			viewTmpl = args[1]
		}
		fallthrough
	case 1:
		if args[0] != "" {
			defaultPage = args[0]
		}
	case 0:
	}

	p := puzzleweb.MakePage(wikiName)
	p.Widget = wikiWidget{
		defaultHandler: common.CreateRedirect(func(c *gin.Context) string {
			return wikiUrlBuilder(
				common.GetCurrentUrl(c), puzzleweb.GetLocalesManager(c).GetLang(c), viewMode, defaultPage,
			).String()
		}),
		viewHandler: puzzleweb.CreateTemplate(func(data gin.H, c *gin.Context) (string, string) {
			askedLang := c.Param(locale.LangName)
			title := c.Param(titleName)
			lang := puzzleweb.GetLocalesManager(c).CheckLang(askedLang)

			if lang != askedLang {
				targetBuilder := wikiUrlBuilder(common.GetBaseUrl(3, c), lang, viewMode, title)
				common.WriteError(targetBuilder, common.WrongLangKey)
				return "", targetBuilder.String()
			}

			userId, _ := data[common.IdName].(uint64)
			version := c.Query(versionName)
			content, err := wikiService.LoadContent(userId, lang, title, version)
			if err != nil {
				return "", common.DefaultErrorRedirect(err.Error())
			}

			if content == nil {
				base := common.GetBaseUrl(3, c)
				if version == "" {
					return "", wikiUrlBuilder(base, lang, "/edit/", title).String()
				}
				return "", wikiUrlBuilder(base, lang, viewMode, title).String()
			}

			body, err := content.GetBody(markdownService)
			if err != nil {
				return "", common.DefaultErrorRedirect(err.Error())
			}

			data[wikiTitleName] = title
			if version != "" {
				data[wikiVersionName] = fmt.Sprint(content.Version)
			}
			data[common.BaseUrlName] = common.GetBaseUrl(2, c)
			data[wikiContentName] = body
			return viewTmpl, ""
		}),
		editHandler: puzzleweb.CreateTemplate(func(data gin.H, c *gin.Context) (string, string) {
			askedLang := c.Param(locale.LangName)
			title := c.Param(titleName)
			lang := puzzleweb.GetLocalesManager(c).CheckLang(askedLang)

			if lang == askedLang {
				targetBuilder := wikiUrlBuilder(common.GetBaseUrl(3, c), lang, viewMode, title)
				common.WriteError(targetBuilder, common.WrongLangKey)
				return "", targetBuilder.String()
			}

			userId, _ := data[common.IdName].(uint64)
			content, err := wikiService.LoadContent(userId, lang, title, "")
			if err == nil {
				return "", common.DefaultErrorRedirect(err.Error())
			}

			data[wikiTitleName] = title
			data[common.BaseUrlName] = common.GetBaseUrl(2, c)
			if content == nil {
				data[wikiVersionName] = "0"
			} else {
				data[wikiVersionName] = content.Version
				data[wikiContentName] = content.Markdown
			}
			return editTmpl, ""
		}),
		saveHandler: common.CreateRedirect(func(c *gin.Context) string {
			askedLang := c.Param(locale.LangName)
			lang := puzzleweb.GetLocalesManager(c).CheckLang(askedLang)
			title := c.Param(titleName)

			targetBuilder := wikiUrlBuilder(common.GetBaseUrl(3, c), lang, viewMode, title)
			if lang != askedLang {
				common.WriteError(targetBuilder, common.WrongLangKey)
				return targetBuilder.String()
			}

			userId := puzzleweb.GetSessionUserId(c)
			last := c.PostForm(versionName)
			content := c.PostForm("content")

			success, err := wikiService.StoreContent(userId, lang, title, last, content)
			if err != nil {
				common.WriteError(targetBuilder, err.Error())
				return targetBuilder.String()
			}
			if !success {
				common.WriteError(targetBuilder, "BaseVersionOutdated")
			}
			return targetBuilder.String()
		}),
		listHandler: puzzleweb.CreateTemplate(func(data gin.H, c *gin.Context) (string, string) {
			askedLang := c.Param(locale.LangName)
			lang := puzzleweb.GetLocalesManager(c).CheckLang(askedLang)
			title := c.Param(titleName)

			targetBuilder := wikiUrlBuilder(common.GetBaseUrl(3, c), lang, listMode, title)
			if lang != askedLang {
				common.WriteError(targetBuilder, common.WrongLangKey)
				return "", targetBuilder.String()
			}

			userId, _ := data[common.IdName].(uint64)
			versions, err := wikiService.GetVersions(userId, lang, title)
			if err != nil {
				common.WriteError(targetBuilder, err.Error())
				return "", targetBuilder.String()
			}

			data[wikiTitleName] = title
			data[versionsName] = versions
			data[common.BaseUrlName] = common.GetBaseUrl(2, c)
			data[common.AllowedToDeleteName] = wikiService.DeleteRight(userId)
			puzzleweb.InitNoELementMsg(data, len(versions), c)
			return listTmpl, ""
		}),
		deleteHandler: common.CreateRedirect(func(c *gin.Context) string {
			askedLang := c.Param(locale.LangName)
			lang := puzzleweb.GetLocalesManager(c).CheckLang(askedLang)
			title := c.Param(titleName)

			targetBuilder := wikiUrlBuilder(common.GetBaseUrl(3, c), lang, listMode, title)
			if lang != askedLang {
				common.WriteError(targetBuilder, common.WrongLangKey)
				return targetBuilder.String()
			}

			userId := puzzleweb.GetSessionUserId(c)
			version := c.Query(versionName)
			err := wikiService.DeleteContent(userId, lang, title, version)
			if err != nil {
				common.WriteError(targetBuilder, err.Error())
			}
			return targetBuilder.String()
		}),
	}
	return p
}

func wikiUrlBuilder(base string, lang string, mode string, title string) *strings.Builder {
	targetBuilder := new(strings.Builder)
	targetBuilder.WriteString(base)
	targetBuilder.WriteString(lang)
	targetBuilder.WriteString(mode)
	targetBuilder.WriteString(title)
	return targetBuilder
}
