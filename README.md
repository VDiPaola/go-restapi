GET
```
localhost:8080/polygons
localhost:8080/polygons/:name
localhost:8080/polygons/generate
```
POST
```
localhost:8080/polygons
{
    "points":[{"x":1, "y":2},{"x":1, "y":5},{"x":5, "y":5},{"x":5, "y":2}],
    "name": "rect"
}
```

### polygon boundaries
- must give vertices in clockwise rotation
- the final closing vertex will automatically be added for you
- x/y must be between 0 and 999999
- polygons must not overlap any existing ones



#### issues:
- area doesnt calculate properly on most randomly generated polygons
- limited generation to 100 per request due to issues
- might need to try requests multiple times if db connection not alive