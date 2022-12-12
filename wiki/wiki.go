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
	"html/template"
	"net/url"
	"strings"

	"github.com/dvaumoron/puzzleweb"
	rightclient "github.com/dvaumoron/puzzleweb/admin/client"
	"github.com/dvaumoron/puzzleweb/errors"
	"github.com/dvaumoron/puzzleweb/locale"
	"github.com/dvaumoron/puzzleweb/log"
	"github.com/dvaumoron/puzzleweb/login"
	"github.com/dvaumoron/puzzleweb/wiki/cache"
	"github.com/dvaumoron/puzzleweb/wiki/client"
	"github.com/gin-gonic/gin"
)

const versionName = "version"
const versionsName = "Versions"
const viewMode = "/view/"
const listMode = "/list/"
const titleName = "title"
const wikiTitleName = "WikiTitle"
const wikiVersionName = "WikiVersion"
const wikiBaseUrlName = "WikiBaseUrl"
const wikiContentName = "WikiContent"

type VersionDisplay struct {
	Title          string
	Number         string
	Creator        string
	BaseUrl        string
	ViewLinkName   string
	DeleteLinkName string
}

type wikiWidget struct {
	defaultHandler gin.HandlerFunc
	viewHandler    gin.HandlerFunc
	editHandler    gin.HandlerFunc
	saveHandler    gin.HandlerFunc
	listHandler    gin.HandlerFunc
	deleteHandler  gin.HandlerFunc
}

func (w *wikiWidget) LoadInto(router gin.IRouter) {
	router.GET("/", w.defaultHandler)
	router.GET("/:lang/view/:title", w.viewHandler)
	router.GET("/:lang/edit/:title", w.editHandler)
	router.POST("/:lang/save/:title", w.saveHandler)
	router.GET("/:lang/list/:title", w.listHandler)
	router.GET("/:lang/delete/:title", w.deleteHandler)
}

func NewWikiPage(wikiName string, wikiId uint64, args ...string) *puzzleweb.Page {
	rightclient.RegisterObject(wikiId, wikiName)
	cache.InitWikiId(wikiId)

	defaultPage := "Welcome"
	viewTmpl := "wiki/view.html"
	editTmpl := "wiki/edit.html"
	listTmpl := "wiki/list.html"
	switch len(args) {
	default:
		log.Logger.Info("NewWikiPage should be called with 2 to 6 arguments.")
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

	p := puzzleweb.NewPage(wikiName)
	p.Widget = &wikiWidget{
		defaultHandler: puzzleweb.CreateRedirect(func(c *gin.Context) string {
			return wikiUrlBuilder(
				puzzleweb.GetCurrentUrl(c), locale.GetLang(c), viewMode, defaultPage,
			).String()
		}),
		viewHandler: puzzleweb.CreateTemplate(func(data gin.H, c *gin.Context) (string, string) {
			askedLang := c.Param(locale.LangName)
			title := c.Param(titleName)
			lang := locale.CheckLang(askedLang)

			redirect := ""
			if lang == askedLang {
				userId := login.GetUserId(c)
				version := c.Query(versionName)
				content, err := client.LoadContent(wikiId, userId, lang, title, version)
				if err == nil {
					if content == nil {
						base := puzzleweb.GetBaseUrl(3, c)
						if version == "" {
							redirect = wikiUrlBuilder(base, lang, "/edit/", title).String()
						} else {
							redirect = wikiUrlBuilder(base, lang, viewMode, title).String()
						}
					} else {
						var body template.HTML
						body, err = content.GetBody()
						if err == nil {
							data[wikiTitleName] = title
							contentVersionStr := fmt.Sprint(content.Version)
							if version == contentVersionStr {
								data["EditLinkName"] = locale.GetText("edit.link.name", c)
							} else {
								data[wikiVersionName] = contentVersionStr
							}
							data["ListLinkName"] = locale.GetText("list.link.name", c)
							data[wikiBaseUrlName] = puzzleweb.GetBaseUrl(2, c)
							data[wikiContentName] = body
						} else {
							redirect = errors.DefaultErrorRedirect(err.Error(), c)
						}
					}
				} else {
					redirect = errors.DefaultErrorRedirect(err.Error(), c)
				}
			} else {
				targetBuilder := wikiUrlBuilder(
					puzzleweb.GetBaseUrl(3, c), lang, viewMode, title,
				)
				writeError(targetBuilder, errors.WrongLang, c)
				redirect = targetBuilder.String()
			}
			return viewTmpl, redirect
		}),
		editHandler: puzzleweb.CreateTemplate(func(data gin.H, c *gin.Context) (string, string) {
			askedLang := c.Param(locale.LangName)
			title := c.Param(titleName)
			lang := locale.CheckLang(askedLang)

			redirect := ""
			if lang == askedLang {
				userId := login.GetUserId(c)
				content, err := client.LoadContent(wikiId, userId, lang, title, "")
				if err == nil {
					data["EditTitle"] = locale.GetText("edit.title", c)
					data[wikiTitleName] = title
					data[wikiBaseUrlName] = puzzleweb.GetBaseUrl(2, c)
					data["CancelLinkName"] = locale.GetText("cancel.link.name", c)
					if content == nil {
						data[wikiVersionName] = "0"
					} else {
						data[wikiContentName] = content.Markdown
						data[wikiVersionName] = content.Version
					}
					data["SaveLinkName"] = locale.GetText("save.link.name", c)
				} else {
					redirect = errors.DefaultErrorRedirect(err.Error(), c)
				}
			} else {
				targetBuilder := wikiUrlBuilder(puzzleweb.GetBaseUrl(3, c), lang, viewMode, title)
				writeError(targetBuilder, errors.WrongLang, c)
				redirect = targetBuilder.String()
			}
			return editTmpl, redirect
		}),
		saveHandler: puzzleweb.CreateRedirect(func(c *gin.Context) string {
			askedLang := c.Param(locale.LangName)
			title := c.Param(titleName)
			lang := locale.CheckLang(askedLang)

			targetBuilder := wikiUrlBuilder(puzzleweb.GetBaseUrl(3, c), lang, viewMode, title)
			if lang == askedLang {
				content := c.PostForm("content")
				last := c.PostForm(versionName)

				userId := login.GetUserId(c)
				err := client.StoreContent(wikiId, userId, lang, title, last, content)
				if err != nil {
					writeError(targetBuilder, err.Error(), c)
				}
			} else {
				writeError(targetBuilder, errors.WrongLang, c)
			}
			return targetBuilder.String()
		}),
		listHandler: puzzleweb.CreateTemplate(func(data gin.H, c *gin.Context) (string, string) {
			askedLang := c.Param(locale.LangName)
			title := c.Param(titleName)
			lang := locale.CheckLang(askedLang)

			redirect := ""
			if lang == askedLang {
				userId := login.GetUserId(c)
				versions, err := client.GetVersions(wikiId, userId, lang, title)
				if err == nil {
					data[wikiTitleName] = title
					size := len(versions)
					if size == 0 {
						data[errors.Msg] = locale.GetText(errors.NoElement, c)
						data[versionsName] = versions
					} else {
						viewLinkName := locale.GetText("view.link.name", c)
						deleteLinkName := locale.GetText("delete.link.name", c)

						baseUrl := puzzleweb.GetBaseUrl(2, c)
						converted := make([]*VersionDisplay, 0, size)
						for _, version := range versions {
							converted = append(converted, &VersionDisplay{
								Title: title, Number: fmt.Sprint(version.Number),
								Creator: version.UserLogin, BaseUrl: baseUrl,
								ViewLinkName: viewLinkName, DeleteLinkName: deleteLinkName,
							})
						}
						data[versionsName] = converted
					}
				} else {
					targetBuilder := wikiUrlBuilder(
						puzzleweb.GetBaseUrl(3, c), lang, listMode, title,
					)
					writeError(targetBuilder, err.Error(), c)
					redirect = targetBuilder.String()
				}
			} else {
				targetBuilder := wikiUrlBuilder(
					puzzleweb.GetBaseUrl(3, c), lang, listMode, title,
				)
				writeError(targetBuilder, errors.WrongLang, c)
				redirect = targetBuilder.String()
			}
			return listTmpl, redirect
		}),
		deleteHandler: puzzleweb.CreateRedirect(func(c *gin.Context) string {
			askedLang := c.Param(locale.LangName)
			title := c.Param(titleName)
			lang := locale.CheckLang(askedLang)

			targetBuilder := wikiUrlBuilder(puzzleweb.GetBaseUrl(3, c), lang, listMode, title)
			if lang == askedLang {
				userId := login.GetUserId(c)
				version := c.Query(versionName)
				err := client.DeleteContent(wikiId, userId, lang, title, version)
				if err != nil {
					writeError(targetBuilder, err.Error(), c)
				}
			} else {
				writeError(targetBuilder, errors.WrongLang, c)
			}
			return targetBuilder.String()
		}),
	}
	return p
}

func wikiUrlBuilder(base, lang, mode, title string) *strings.Builder {
	var targetBuilder strings.Builder
	targetBuilder.WriteString(base)
	targetBuilder.WriteString(lang)
	targetBuilder.WriteString(mode)
	targetBuilder.WriteString(title)
	return &targetBuilder
}

func writeError(urlBuilder *strings.Builder, errMsg string, c *gin.Context) {
	urlBuilder.WriteString(errors.QueryError)
	urlBuilder.WriteString(url.QueryEscape(locale.GetText(errMsg, c)))
}
