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
package blog

import (
	"github.com/dvaumoron/puzzleweb"
	"github.com/gin-gonic/gin"
)

type blogWidget struct {
	listHandler    gin.HandlerFunc
	viewHandler    gin.HandlerFunc
	commentHandler gin.HandlerFunc
	createHandler  gin.HandlerFunc
	previewHandler gin.HandlerFunc
	saveHandler    gin.HandlerFunc
	deleteHandler  gin.HandlerFunc
}

func (w *blogWidget) LoadInto(router gin.IRouter) {
	router.GET("/", w.listHandler)
	router.GET("/view/:postId", w.viewHandler)
	router.POST("/comment/:postId", w.commentHandler)
	router.GET("/create/", w.createHandler)
	router.POST("/preview/", w.previewHandler)
	router.POST("/save/", w.saveHandler)
	router.GET("/delete/:postId", w.deleteHandler)
}

func NewBlogPage(blogName string, groupId uint64, blogId uint64, args ...string) *puzzleweb.Page {
	// TODO
	p := puzzleweb.NewPage(blogName)
	p.Widget = &blogWidget{}
	return p
}
