package build

import  (

     "fmt"

     "github.com/docker/docker/api/server/router"
)


// buildRouter is a router to talk with the build controller
type buildRouter struct {
	backend Backend
	routes  []router.Route
}

// NewRouter initializes a new build router
func NewRouter(b Backend) router.Router {
    fmt.Println("api/server/router/build/build.go  NewRouter()")
	r := &buildRouter{
		backend: b,
	}
	r.initRoutes()
	return r
}

// Routes returns the available routers to the build controller
func (r *buildRouter) Routes() []router.Route {
    fmt.Println("api/server/router/build/build.go  Routes()")
	return r.routes
}

func (r *buildRouter) initRoutes() {
	r.routes = []router.Route{
		router.Cancellable(router.NewPostRoute("/build", r.postBuild)),
	}
}
