package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/go-sql-driver/mysql"
)

var db *sql.DB

func dbInit() {
	dbConnect()
	dbPing()
}

func dbConnect() error {
	//config
	cfg := mysql.Config{
		User:                 os.Getenv("DBUSER"),
		Passwd:               os.Getenv("DBPASS"),
		Net:                  "tcp",
		Addr:                 os.Getenv("DBHOST") + ":" + os.Getenv("DBPORT"),
		DBName:               os.Getenv("DBNAME"),
		AllowNativePasswords: true,
	}

	// Get a database handle.
	var err error
	db, err = sql.Open("mysql", cfg.FormatDSN())

	return err
}

func dbPing() error {
	//check connection
	pingErr := db.Ping()
	if pingErr != nil {
		return pingErr
	}

	//prepare table
	_, err := db.Exec("CREATE TABLE IF NOT EXISTS `enzo_challenge` (" +
		"`ID` INT NOT NULL AUTO_INCREMENT," +
		"`name` TEXT NOT NULL," +
		"`polygon` POLYGON NOT NULL," +
		"`area` INT NOT NULL," +
		"`points` TEXT NOT NULL," +
		"PRIMARY KEY (`ID`));")

	return err
}

//inserts a row into the table
func dbAddPolygon(p polygon) error {
	//check db connection is alive
	if pingErr := dbPing(); pingErr != nil {
		return fmt.Errorf("dbAddPolygon: %v", pingErr)
	}

	//convert points to json
	points, err := json.Marshal(p.Points)
	if err != nil {
		return fmt.Errorf("dbAddPolygon: %v", err)
	}
	//insert row
	q := fmt.Sprintf("INSERT INTO enzo_challenge (polygon, area, name, points) VALUES (ST_GeomFromText('POLYGON((%s))'), %.1f, '%s', '%s')", formatPoints(p.Points), p.Area, p.Name, points)
	_, err2 := db.Exec(q)
	if err2 != nil {
		return fmt.Errorf("dbAddPolygon: %v", err2)
	}
	return nil
}

//insert multiple polygons in 1 statement
func dbAddPolygons(p *[]polygon) error {
	//check db connection is alive
	if pingErr := dbPing(); pingErr != nil {
		return fmt.Errorf("dbAddPolygons: %v", pingErr)
	}

	//format polygons and execute query
	multiPolys := multiPolygonFormatter(p)
	q := fmt.Sprintf("INSERT INTO enzo_challenge (polygon, area, name, points) VALUES %s", multiPolys)
	_, err2 := db.Exec(q)
	if err2 != nil {
		return fmt.Errorf("dbAddPolygons: %v", err2)
	}
	return nil
}

func multiPolygonFormatter(polygons *[]polygon) string {
	var strArr []string
	for _, p := range *polygons {
		//convert points to json
		if points, err := json.Marshal(p.Points); err == nil {
			//append string
			strArr = append(strArr, fmt.Sprintf("(ST_GeomFromText('POLYGON((%s))'), %.1f, '%s', '%s')", formatPoints(p.Points), p.Area, p.Name, points))
		}
	}
	return strings.Join(strArr, ",")
}

//formats point array so it can be used with POLYGON data type e.g. x y, x y
func formatPoints(points []point) string {
	var formatedString []string
	for _, point := range points {
		formatedString = append(formatedString, fmt.Sprintf("%.1f %.1f", point.X, point.Y))
	}
	return strings.Join(formatedString, ",")
}

//gets all the rows from the table
func dbGetPolygons() ([]polygon, error) {
	//check db connection is alive
	if pingErr := dbPing(); pingErr != nil {
		return nil, fmt.Errorf("dbGetPolygons: %v", pingErr)
	}

	var polygons []polygon
	//get rows
	rows, err := db.Query("SELECT name,points,area FROM enzo_challenge")
	if err != nil {
		return nil, fmt.Errorf("dbGetPolygons: %v", err)
	}
	defer rows.Close()

	// Loop through rows, using Scan to assign column data to struct fields.
	for rows.Next() {
		var p polygon
		var points string

		if err := rows.Scan(&p.Name, &points, &p.Area); err != nil {
			return nil, fmt.Errorf("dbGetPolygons: %v", err)
		}

		//convert points string to []point
		if err := json.Unmarshal([]byte(points), &p.Points); err != nil {
			return nil, fmt.Errorf("dbGetPolygons: %v", err)
		}

		polygons = append(polygons, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("dbGetPolygons: %v", err)
	}

	return polygons, nil
}

//checks if the provided points have intersections with polygons in database
func dbCheckIntersections(points *[]point) (bool, error) {
	//check db connection is alive
	if pingErr := dbPing(); pingErr != nil {
		return false, fmt.Errorf("dbCheckIntersections: %v", pingErr)
	}

	//get intersections using built in function
	q := fmt.Sprintf("SELECT COUNT(*) FROM  `enzo_challenge` WHERE ST_Intersects(`polygon`, ST_GeomFromText('POLYGON((%s))') );", formatPoints(*points))
	rows, err2 := db.Query(q)
	if err2 != nil {
		return false, fmt.Errorf("dbCheckIntersections: %v", err2)
	}

	defer rows.Close()

	var count int

	//get count from rows
	for rows.Next() {
		if err := rows.Scan(&count); err != nil {
			return false, fmt.Errorf("dbCheckIntersections: %v", err)
		}
	}

	if count > 0 {
		return true, nil
	}
	return false, nil
}

//checks if a given name is found in the database
func dbNameExists(name string) (bool, error) {
	//check db connection is alive
	if pingErr := dbPing(); pingErr != nil {
		return false, fmt.Errorf("dbNameExists: %v", pingErr)
	}

	//query database for count
	rows, err := db.Query("SELECT COUNT(*) FROM enzo_challenge WHERE name=?", name)
	if err != nil {
		return false, fmt.Errorf("dbNameExists: %v", err)
	}
	defer rows.Close()

	var count int

	//get count from rows
	for rows.Next() {
		if err := rows.Scan(&count); err != nil {
			return false, fmt.Errorf("dbNameExists: %v", err)
		}
	}

	if count > 0 {
		return true, nil
	}
	return false, nil
}


