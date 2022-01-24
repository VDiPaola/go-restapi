package main

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type point struct {
	X float32 `min:"0" max:"999999"`
	Y float32
}

type polygon struct {
	Points []point `json:"points"`
	Area   float32 `json:"area"`
	Name   string  `json:"name"`
}

// cached polygons
var polygonsCache []polygon

func main() {
	rand.Seed(time.Now().UnixNano())

	//database
	dbInit()
	updateCache()

	//router
	router := gin.Default()
	router.GET("/polygons", getPolygons)
	router.GET("/polygons/:name", getPolygonByName)
	router.POST("/polygons", postPolygons)

	router.GET("/polygons/generate", polygonGenerator)

	if err := router.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}

// get all polygons
func getPolygons(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, polygonsCache)
}

// adds polygon to database
func postPolygons(c *gin.Context) {
	var newPolygon polygon

	// bind receieved json to polygon struct
	if err := c.BindJSON(&newPolygon); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "cant create polygon from this"})
		return
	}

	//validate
	if err := addPolygon(c, &newPolygon); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	// Add polygon to database
	err := dbAddPolygon(newPolygon)
	if err != nil {
		fmt.Println(err.Error())
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
	} else {
		//update cache
		updateCache()
		//return new polygon
		c.IndentedJSON(http.StatusCreated, newPolygon)
	}
}

func addPolygon(c *gin.Context, newPolygon *polygon) error {
	//check they have atleast 3 vertices
	if len(newPolygon.Points) < 3 {
		return fmt.Errorf("addPolygon: you must have atleast 3 vertices")
	}

	//make sure name is unique
	if exists, err := dbNameExists(newPolygon.Name); err != nil {
		return fmt.Errorf("addPolygon: %v", err)
	} else if exists {
		return fmt.Errorf("addPolygon: polygon with that name already exists")
	}

	//add first vertices to end to complete shape then format array
	newPolygon.Points = append(newPolygon.Points, newPolygon.Points[0])
	reverse(&newPolygon.Points)

	//check for intersections
	if hasIntersections, err := dbCheckIntersections(&newPolygon.Points); err != nil {
		return fmt.Errorf("addPolygon: %v", err)
	} else if hasIntersections {
		return fmt.Errorf("addPolygon: polygon is intersecting with others in database")
	}

	//get area from points
	var valid bool
	newPolygon.Area, valid = getArea(newPolygon.Points)

	if !valid {
		return fmt.Errorf("addPolygon: x and y bounds not satisfied")
	}

	return nil
}

//checks if points are within the min/max boundaries
func verifyPoint(point *point) bool {
	t := reflect.TypeOf(*point)
	//get min max tags
	for i := 0; i < t.NumField(); i++ {
		//min
		if minString, ok := t.Field(i).Tag.Lookup("min"); ok {
			if min, err := strconv.ParseFloat(minString, 32); err == nil {
				if point.X < float32(min) || point.Y < float32(min) {
					return false
				}
			}
		}
		//max
		if maxString, ok := t.Field(i).Tag.Lookup("max"); ok {
			if max, err := strconv.ParseFloat(maxString, 32); err == nil {
				if point.X > float32(max) || point.Y > float32(max) {
					return false
				}
			}
		}
	}

	return true
}

//get area of polygon given full list of vertices in anti-clockwise rotation
func getArea(points []point) (float32, bool) {
	var xySum float32
	var yxSum float32
	for i := 0; i < len(points)-1; i++ {
		//check points are within min/max bounds
		if !verifyPoint(&points[i]) {
			return 0., false
		}
		xySum += points[i].X * points[i+1].Y
		yxSum += points[i].Y * points[i+1].X
	}
	return (xySum - yxSum) / 2, true
}

//reverses point array
func reverse(points *[]point) {
	for i := 0; i < len(*points)/2; i++ {
		j := len(*points) - i - 1
		(*points)[i], (*points)[j] = (*points)[j], (*points)[i]
	}
}

func updateCache() {
	polygons, err := dbGetPolygons()
	if err != nil {
		//log.Fatal(err)
		fmt.Println(err)
	} else {
		polygonsCache = polygons
	}
}

// get polygons by name
func getPolygonByName(c *gin.Context) {
	name := c.Param("name")

	// look for match for name
	for _, p := range polygonsCache {
		if p.Name == name {
			c.IndentedJSON(http.StatusOK, p)
			return
		}
	}
	c.IndentedJSON(http.StatusNotFound, gin.H{"message": "polygon not found"})
}

func polygonGenerator(c *gin.Context) {
	//use waitgroup so update cache happens after generation
	var wg sync.WaitGroup

	id := randomNumber(0, 1000000)

	var polygons []polygon

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			//create polygon
			var newPoly polygon
			newPoly.Name = "randomPoly_" + fmt.Sprint(id) + "_" + fmt.Sprint(i)
			newPoly.Points = generateVertices(1, 100, uint32(randomNumber(3, 20)))
			reverse(&newPoly.Points)
			//validate
			if err := addPolygon(c, &newPoly); err != nil {
				c.IndentedJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
				return
			}

			// Add polygon to array
			polygons = append(polygons, newPoly)
		}(i)
	}

	wg.Wait()
	//add to db
	dbAddPolygons(&polygons)

	//update cache
	updateCache()
	c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "Finished generation"})
}

//generates an array of vertices
func generateVertices(minSize uint32, maxSize uint32, amount uint32) []point {
	//get random starting point
	offset := randomNumber(1, 999998)

	var vertices []point

	//radians split by amount of vertices wanted
	spread := 2 * math.Pi / float32(amount+1)

	var i uint32
	for ; i < amount; i++ {
		angle := randomNumber(float32(i)*spread, float32((i+1))*spread)
		//get x/y from angle * distance + offset
		x := float32(math.Cos(float64(angle))) * (randomNumber(float32(minSize), float32(maxSize)))
		y := float32(math.Sin(float64(angle))) * (randomNumber(float32(minSize), float32(maxSize)))
		x += offset
		y += offset
		//lock numbers to min max
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		if x > 999999 {
			x = 999999
		}
		if y > 999999 {
			y = 999999
		}
		//append to array
		vertices = append(vertices, point{
			X: x,
			Y: y,
		})

	}

	return vertices
}

//generate random number
func randomNumber(from float32, to float32) float32 {
	return (rand.Float32() * (to - from)) + from
}
