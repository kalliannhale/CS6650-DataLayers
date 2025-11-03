package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

// constants
const DataSize = 100000
const CartStatusActive = "active"

// database pointer
var db *sql.DB

// define Product struct
type Product struct {
	ID            int32  `json:"product_id" binding:"required,gte=1"`
	Name          string `json:"name" binding:"required,min=1"`
	Category      string `json:"category" binding:"required,min=1"`
	Description   string `json:"description" binding:"required,min=1"`
	Brand         string `json:"brand" binding:"required, min=1"`
	NameLower     string `json:"-"` // used for search, excluded in response
	CategoryLower string `json:"-"` // used for search, excluded in response
}

// define SearchResult struct
type SearchResult struct {
	Products   []*Product `json:"products"`
	TotalFound int        `json:"total_found"`
	SearchTime string     `json:"search_time"`
}

// define read shopping cart structs
type cartItemInfo struct {
	ProductID   int32  `json:"product_id"`
	ProductName string `json:"product_name"`
	Quantity    int64  `json:"quantity"`
}
type shoppingCartInfo struct {
	CartID     uint64         `json:"cart_id"`
	CustomerID uint64         `json:"customer_id"`
	Status     string         `json:"status"`
	Items      []cartItemInfo `json:"items"`
}

// define update shopping cart structs
type updateCartItem struct {
	ProductID int32 `json:"product_id"`
	Quantity  uint  `json:"quantity"`
}
type updateCartItemsRequest struct {
	Items []updateCartItem `json:"items"`
}

// define Error struct
type ErrorResponse struct {
	Err     string `json:"error"`
	Message string `json:"message"`
	Details string `json:"details"`
}

func main() {
	// Initialize database
	InitDB()
	defer db.Close()

	// Generate 100,000 products if the database is empty
	var count int
	db.QueryRow("SELECT COUNT(*) FROM product").Scan(&count)
	if count == 0 {
		generateData(DataSize)
	}

	// Router
	router := gin.Default()

	// Product service endpoints
	router.GET("/products/:productId", getProduct)
	router.POST("/products/:productId/details", addProductDetails)
	router.GET("/products/search", search)

	// Shopping cart service endpoints
	router.POST("/shopping-carts", createShoppingCart)
	router.GET("/shopping-carts/:id", getShoppingCart)
	router.POST("/shopping-carts/:id/items", updateItemToShoppingCart)

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	// Debug endpoins
	router.DELETE("/debug/clear-carts", clearCartsData)

	// router.Run("<ip_addr>:8080")
	router.Run(":8080")
}

// Product service endpoints
/* Get product by ID: retrieve a product's details using its unique identifier */
func getProduct(c *gin.Context) {
	// retrieve productID from the path
	productIDStr := c.Param("productId")

	// check input productID validation
	productID, err := strconv.ParseInt(productIDStr, 10, 32)
	if (err != nil) || (productID < 1) {
		err404 := ErrorResponse{
			Err:     "PRODUCT_NOT_FOUND",
			Message: "Product not found",
			Details: fmt.Sprintf("Invalid input: product ID must be an positive integer >= 1 (input: %s)", productIDStr),
		}
		c.JSON(http.StatusNotFound, err404) // status 404 + Error
		return
	}
	productIDInt32 := int32(productID)

	// look for Product by productID in database
	p, err := queryProductByID(productIDInt32)

	if err == sql.ErrNoRows {
		err404 := ErrorResponse{
			Err:     "PRODUCT_NOT_FOUND",
			Message: "Product not found",
			Details: fmt.Sprintf("No product found for ID %d", productIDInt32),
		}
		c.JSON(http.StatusNotFound, err404) // status 404 + Error
		return
	} else if err != nil {
		err500 := ErrorResponse{
			Err:     "INTERNAL_ERROR",
			Message: "Fail to query product in the database",
			Details: err.Error(),
		}
		c.JSON(http.StatusInternalServerError, err500)
		return
	}

	c.JSON(http.StatusOK, p)
}

/* Add product details: add or update detailed information for a specific product
 * Assumption: I assume that this POST request updates a product if it exists, and returns 404 if the specified product ID does not exist. */
func addProductDetails(c *gin.Context) {
	// retrieve productID from the path
	productIDStr := c.Param("productId")

	// check productID validation
	productID, err := strconv.ParseInt(productIDStr, 10, 32)
	if (err != nil) || (productID < 1) {
		err400 := ErrorResponse{
			Err:     "INVALID_INPUT",
			Message: "The provided input product ID is invalid",
			Details: fmt.Sprintf("Product ID must be an positive integer >= 1 (input: %s)", productIDStr),
		}
		c.JSON(http.StatusBadRequest, err400) // status 400 + Error
		return
	}
	productIDInt32 := int32(productID)

	// retrieve request body
	var newProductDetails Product
	if err := c.BindJSON(&newProductDetails); err != nil {
		err400 := ErrorResponse{
			Err:     "INVALID_INPUT",
			Message: "The provided request body is invalid",
			Details: fmt.Sprintf("%v", err),
		}
		c.JSON(http.StatusBadRequest, err400) // status 400 + Error
		return
	}

	// check request body
	if newProductDetails.ID != productIDInt32 {
		err400 := ErrorResponse{
			Err:     "INVALID_INPUT",
			Message: "The provided request body is invalid",
			Details: "The product_id in the request body is different from product_id indicated in the path",
		}
		c.JSON(http.StatusBadRequest, err400) // status 400 + Error
		return
	}

	newProductDetails.NameLower = strings.ToLower(newProductDetails.Name)
	newProductDetails.CategoryLower = strings.ToLower(newProductDetails.Category)

	// Check if product exists
	err = updateProductIfExists(newProductDetails)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			err404 := ErrorResponse{
				Err:     "PRODUCT_NOT_FOUND",
				Message: "Product not found",
				Details: fmt.Sprintf("No product found for ID %d", productIDInt32),
			}
			c.JSON(http.StatusNotFound, err404) // status 404 + Error
			return
		}
		err500 := ErrorResponse{
			Err:     "DB_ERROR",
			Message: "Failed to update product in the database",
			Details: err.Error(),
		}
		c.JSON(http.StatusInternalServerError, err500)
		return
	}

	c.Status(http.StatusNoContent) // status 204
}

/* Search: search products in terms of "name" and "category" based on queries
 * 	Search criteria:
 * 		- /products/search                       queryMethod = "both" - no search criteria
 * 		- /products/search?q=xxx                 queryMethod = "both" - fuzzy search in both "name" and "category" fields
 *   	note: any other query parameter will be ignored */
func search(c *gin.Context) {
	genQuery := c.Query("q")
	response, err := searchInNameCategory(genQuery, 100, 20)
	if err != nil {
		err500 := ErrorResponse{
			Err:     "DB_ERROR",
			Message: "Failed to search products",
			Details: err.Error(),
		}
		c.JSON(http.StatusInternalServerError, err500)
		return
	}
	c.JSON(http.StatusOK, response)
}

/* Internal function to search products in terms of "name" and "category" with bounded iteration */
func searchInNameCategory(query string, searchLimit int, resultLimit int) (SearchResult, error) {
	start := time.Now()

	// prepare for search
	productsFound := make([]*Product, 0, resultLimit)
	totalFound := 0
	queryLower := strings.ToLower(query)

	// Initialize a starting index for search and ensure the searchLimit does not exceed the datasize
	var totalRecords int
	err := db.QueryRow(`SELECT COUNT(*) FROM product`).Scan(&totalRecords)
	if err != nil {
		return SearchResult{}, err
	}

	var startIdx int
	if totalRecords > searchLimit {
		startIdx = rand.Intn(totalRecords - searchLimit)
	} else {
		startIdx = 0
		searchLimit = totalRecords
	}

	// get the data for search
	// var rows *sql.Rows
	// if queryLower == "" {
	// 	sqlQuery := `
	// 		SELECT product_id, name, category, brand, description, name_lowercase, category_lowercase
	// 		FROM product
	// 		LIMIT ?, ?
	// 	`
	// 	rows, err = db.Query(sqlQuery, startIdx, searchLimit)
	// } else {
	// 	queryContent := "%" + queryLower + "%"
	// 	sqlQuery := `
	// 		SELECT product_id, name, category, brand, description, name_lowercase, category_lowercase
	// 		FROM product
	// 		WHERE name_lowercase LIKE ? OR category_lowercase LIKE ?
	// 		LIMIT ?, ?
	// 	`
	// 	rows, err = db.Query(sqlQuery, queryContent, queryContent, startIdx, searchLimit)
	// }
	sqlQuery := `
			SELECT product_id, name, category, brand, description, name_lowercase, category_lowercase
			FROM product
			LIMIT ?, ?
		`
	rows, err := db.Query(sqlQuery, startIdx, searchLimit)

	if err != nil {
		return SearchResult{}, err
	}
	defer rows.Close()

	//  traverse and conduct search
	for rows.Next() {
		var p Product
		err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Category,
			&p.Brand,
			&p.Description,
			&p.NameLower,
			&p.CategoryLower,
		)
		if err != nil {
			return SearchResult{}, err
		}

		if strings.Contains(p.NameLower, queryLower) || strings.Contains(p.CategoryLower, queryLower) {
			if totalFound < resultLimit {
				totalFound++
				productsFound = append(productsFound, &p)
			}
		}
	}

	searchTime := time.Since(start).Seconds()

	return SearchResult{
		Products:   productsFound,
		TotalFound: totalFound,
		SearchTime: fmt.Sprintf("%.6fs", searchTime),
	}, nil
}

// Shopping cart service endpoints
/* Creates a new shopping cart and returns the cart ID and initial state
 * Assumption: Only creates an active shopping cart if the customer does not have an active shopping cart
 */
func createShoppingCart(c *gin.Context) {
	// define structures used in this function
	var req struct {
		CustomerID uint64 `json:"customer_id"`
	}
	var cart struct {
		CartID uint64 `json:"cart_id"`
		Status string `json:"status"`
	}

	// parse request content
	if err := c.BindJSON(&req); err != nil {
		err400 := ErrorResponse{
			Err:     "INVALID_INPUT",
			Message: "The provided request body is invalid",
			Details: fmt.Sprintf("%v", err),
		}
		c.JSON(http.StatusBadRequest, err400) // status 400 + Error
		return
	}

	// check format of customer ID
	if req.CustomerID == 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Err:     "INVALID_INPUT",
			Message: "customer_id cannot be 0",
		})
		return
	}

	// create a new shopping cart record in the database if the customer does not have an active cart
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer tx.Rollback()

	var cartID int64
	err = tx.QueryRow(`
        SELECT cart_id 
        FROM shopping_cart 
        WHERE customer_id = ? AND status = 'active'
        FOR UPDATE
    `, req.CustomerID).Scan(&cartID)

	if err != nil && err != sql.ErrNoRows {
		err500 := ErrorResponse{
			Err:     "DB_ERROR",
			Message: "Failed to create shopping cart",
			Details: err.Error(),
		}
		c.JSON(http.StatusInternalServerError, err500)
		return
	}

	if err == nil { // active cart already exists for this customer
		err400 := ErrorResponse{
			Err:     "INVALID_INPUT",
			Message: "Active cart already exists",
			Details: fmt.Sprintf("Customer %d has an active shopping cart (id = %d)", req.CustomerID, cartID),
		}
		c.JSON(http.StatusBadRequest, err400) // status 400 + Error
		return
	}

	res, err := tx.Exec(
		"INSERT INTO shopping_cart (customer_id) VALUES (?)",
		req.CustomerID,
	)
	if err != nil {
		err500 := ErrorResponse{
			Err:     "DB_ERROR",
			Message: "Failed to create shopping cart",
			Details: err.Error(),
		}
		c.JSON(http.StatusInternalServerError, err500)
		return
	}

	// get the return cart_id
	cartID, err = res.LastInsertId()
	if err != nil {
		err500 := ErrorResponse{
			Err:     "DB_ERROR",
			Message: "Failed to retrieve last insert id",
			Details: err.Error(),
		}
		c.JSON(http.StatusInternalServerError, err500)
		return
	}

	if err := tx.Commit(); err != nil {
		err500 := ErrorResponse{
			Err:     "DB_ERROR",
			Message: "Failed to commit",
			Details: err.Error(),
		}
		c.JSON(http.StatusInternalServerError, err500)
		return
	}

	// prepare and send the response
	cart.CartID = uint64(cartID)
	cart.Status = CartStatusActive
	c.JSON(http.StatusCreated, cart)
}

/* Get a shopping cart with its items */
func getShoppingCart(c *gin.Context) {
	// get cartID from the path
	cartIDStr := c.Param("id")

	// check input productID validation
	cartID, err := strconv.ParseUint(cartIDStr, 10, 64)
	if (err != nil) || (cartID < 1) {
		err400 := ErrorResponse{
			Err:     "INVALID_INPUT",
			Message: "The provided shopping cart id is invalid",
			Details: fmt.Sprintf("Shopping cart ID must be an positive integer >= 1 (input: %s)", cartIDStr),
		}
		c.JSON(http.StatusBadRequest, err400) // status 404 + Error
		return
	}

	// prepare response
	var response shoppingCartInfo
	var itemsJSON []byte

	// get shopping cart information and items from the database
	// use JSON_ARRAYAGG to aggregate results into JSON objects directly
	row := db.QueryRow(`
		SELECT 
			sc.cart_id,
			sc.customer_id,
			sc.status,
			COALESCE(
				JSON_ARRAYAGG(
					JSON_OBJECT(
						'product_id', ci.product_id,
						'product_name', p.name,
						'quantity', ci.quantity
					)
				), JSON_ARRAY()
			) AS items
		FROM shopping_cart sc
		LEFT JOIN cart_item ci ON sc.cart_id = ci.cart_id
		LEFT JOIN product p ON ci.product_id = p.product_id
		WHERE sc.cart_id = ?
		GROUP BY sc.cart_id, sc.customer_id, sc.status;
    `, cartID) // query the database once, no transcation needed

	err = row.Scan(&response.CartID, &response.CustomerID, &response.Status, &itemsJSON) //

	if err != nil {
		if err == sql.ErrNoRows {
			err404 := ErrorResponse{
				Err:     "CART_NOT_FOUND",
				Message: "Shopping cart not found",
				Details: fmt.Sprintf("Shopping cart %s does not exist", cartIDStr),
			}
			c.JSON(http.StatusNotFound, err404) // status 404 + Error
			return
		} else {
			err500 := ErrorResponse{
				Err:     "INTERNAL_ERROR",
				Message: "Fail to assign shopping cart itmes to variables",
				Details: err.Error(),
			}
			c.JSON(http.StatusInternalServerError, err500)
			return
		}
	}

	// parse the JSON objects into Go slice
	if err := json.Unmarshal(itemsJSON, &response.Items); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Err:     "INTERNAL_ERROR",
			Message: "Failed to parse cart items JSON",
			Details: err.Error(),
		})
		return
	}

	// response
	c.JSON(http.StatusOK, response)
}

/* Add or update items in existing cart (handle product references and quantities). */
func updateItemToShoppingCart(c *gin.Context) {
	// parse cartID from the path
	cartIDStr := c.Param("id")
	cartID, err := strconv.ParseUint(cartIDStr, 10, 64)
	if (err != nil) || (cartID < 1) {
		err400 := ErrorResponse{
			Err:     "INVALID_INPUT",
			Message: "The provided shopping cart id is invalid",
			Details: fmt.Sprintf("Shopping cart ID must be an positive integer >= 1 (input: %s)", cartIDStr),
		}
		c.JSON(http.StatusBadRequest, err400) // status 400 + Error
		return
	}

	// parse the request
	var req updateCartItemsRequest
	if err := c.BindJSON(&req); err != nil {
		err400 := ErrorResponse{
			Err:     "INVALID_INPUT",
			Message: "The provided request body is invalid",
			Details: err.Error(),
		}
		c.JSON(http.StatusBadRequest, err400) // status 400 + Error
		return
	}

	// check if there are duplicated products in the request
	seen := make(map[int32]bool)
	duplicateProducts := make([]int32, 0)
	productIDs := make([]any, 0, len(req.Items))
	for _, item := range req.Items {
		if seen[item.ProductID] {
			duplicateProducts = append(duplicateProducts, item.ProductID)
		} else {
			seen[item.ProductID] = true
			productIDs = append(productIDs, item.ProductID)
		}
	}
	if len(duplicateProducts) > 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Err:     "INVALID_INPUT",
			Message: "Duplicate product_ids in request",
			Details: fmt.Sprintf("Duplicate product_ids in request: %v", duplicateProducts),
		})
		return
	}

	// start transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Err:     "INTERNAL_ERROR",
			Message: "Failed to start transaction",
			Details: err.Error(),
		})
		return
	}
	defer tx.Rollback()

	// check if the shopping cart exists and lock the shopping cart row
	var exists bool
	err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM shopping_cart WHERE cart_id=? FOR UPDATE)", cartID).Scan(&exists)
	if err != nil {
		err500 := ErrorResponse{
			Err:     "INTERNAL_ERROR",
			Message: "Fail to assign shopping cart item",
			Details: err.Error(),
		}
		c.JSON(http.StatusInternalServerError, err500)
		return
	}
	if !exists {
		err404 := ErrorResponse{
			Err:     "CART_NOT_FOUND",
			Message: "Shopping cart not found",
			Details: fmt.Sprintf("Shopping cart %s  does not exist", cartIDStr),
		}
		c.JSON(http.StatusNotFound, err404) // status 404 + Error
		return
	}

	// check if all product_id listed in the request are valid and lock the involved product rows
	placeholders := strings.TrimRight(strings.Repeat("?,", len(productIDs)), ",")
	productQuery := fmt.Sprintf("SELECT product_id FROM product WHERE product_id IN (%s) FOR UPDATE", placeholders)
	rows, err := tx.Query(productQuery, productIDs...)
	if err != nil {
		err500 := ErrorResponse{
			Err:     "DB_ERROR",
			Message: "Fail to check products in the database",
			Details: err.Error(),
		}
		c.JSON(http.StatusInternalServerError, err500)
		return
	}
	defer rows.Close()

	// check if all products exist
	existingList := make([]int32, 0, len(req.Items))
	existingMap := make(map[int32]bool)
	for rows.Next() {
		var pid int32
		if err := rows.Scan(&pid); err != nil {
			err500 := ErrorResponse{
				Err:     "INTERNAL_ERROR",
				Message: "Fail to assign product id to variable",
				Details: err.Error(),
			}
			c.JSON(http.StatusInternalServerError, err500)
			return
		}
		existingMap[pid] = true
		existingList = append(existingList, pid)
	}

	// return 404 if some product not exist
	if len(existingList) < len(productIDs) {
		missingProducts := make([]int32, 0)
		for _, item := range req.Items {
			if !existingMap[item.ProductID] {
				missingProducts = append(missingProducts, item.ProductID)
			}
		}
		err404 := ErrorResponse{
			Err:     "PRODUCT_NOT_FOUND",
			Message: "Invalid product_id",
			Details: fmt.Sprintf("Product IDs %v not found", missingProducts),
		}
		c.JSON(http.StatusNotFound, err404) // status 404 + Error
		return
	}

	// add or update items
	statement, err := tx.Prepare(`
		INSERT INTO cart_item (product_id, quantity, cart_id)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE quantity = VALUES(quantity)
	`)
	if err != nil {
		err500 := ErrorResponse{
			Err:     "INTERNAL_ERROR",
			Message: "Fail to assign shopping cart item",
			Details: err.Error(),
		}
		c.JSON(http.StatusInternalServerError, err500)
		return
	}
	defer statement.Close()

	for _, item := range req.Items {
		_, err = statement.Exec(item.ProductID, item.Quantity, cartID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to add/update item %d", item.ProductID)})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		err500 := ErrorResponse{
			Err:     "INTERNAL_ERROR",
			Message: "Fail to assign shopping cart item",
			Details: err.Error(),
		}
		c.JSON(http.StatusInternalServerError, err500)
		return
	}

	// response
	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Cart %d updated", cartID),
	})
}

/* Internal function: Initialize database */
func InitDB() {
	// godotenv.Load()

	// get database information from the environment
	username := os.Getenv("DB_USERNAME")
	password := os.Getenv("DB_PASSWORD")
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	name := os.Getenv("DB_NAME")

	if username == "" || password == "" || host == "" || port == "" || name == "" {
		log.Fatal("Database environment variables are not fully set")
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", username, password, host, port, name)

	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("Failed to connect to DB:", err)
	}

	// configure connection pool
	db.SetMaxOpenConns(3) // max connections
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Minute * 5)

	// test connection
	if err := db.Ping(); err != nil {
		log.Fatal("DB ping failed:", err)
	}

	// create tables in the database
	createDBTables()
}

/* Internal function: Create tables in the database */
func createDBTables() {
	// prepare MySQL queries
	productTable := `
	CREATE TABLE IF NOT EXISTS product (
		product_id INT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(255),
		category VARCHAR(255),
		brand VARCHAR(255),
		description TEXT,
		name_lowercase VARCHAR(255),
		category_lowercase VARCHAR(255),
		INDEX idx_name_lower (name_lowercase),
		INDEX idx_category_lower (category_lowercase)
	) ENGINE=InnoDB;`

	shoppingCartTable := `
    CREATE TABLE IF NOT EXISTS shopping_cart (
		cart_id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
		customer_id BIGINT UNSIGNED NOT NULL,
		status ENUM('active','ordered','paid','shipped','completed','cancelled','invalid') NOT NULL DEFAULT 'active',
		INDEX idx_customer_id (customer_id),
		INDEX idx_status_customer (status, customer_id)
    ) ENGINE=InnoDB;`

	cartItemTable := `
	CREATE TABLE IF NOT EXISTS cart_item (
		item_id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
		product_id INT NOT NULL,
		quantity INT UNSIGNED NOT NULL CHECK (quantity > 0), 
		cart_id BIGINT UNSIGNED NOT NULL, 
		status ENUM('valid', 'invalid') NOT NULL DEFAULT 'valid',
		FOREIGN KEY (cart_id) REFERENCES shopping_cart(cart_id) ON DELETE CASCADE,
		FOREIGN KEY (product_id) REFERENCES product(product_id) ON DELETE CASCADE,
		INDEX idx_cart_id (cart_id),
		INDEX idx_product_id (product_id),
		UNIQUE (cart_id, product_id)
	) ENGINE=InnoDB;`

	inventoryTable := `
	CREATE TABLE IF NOT EXISTS inventory (
		product_id INT PRIMARY KEY,
		stock INT UNSIGNED CHECK (stock >= 0),
		reserved INT UNSIGNED CHECK (reserved >= 0),
		FOREIGN KEY (product_id) REFERENCES product(product_id) ON DELETE CASCADE,
		CHECK (reserved <= stock)
	) ENGINE=InnoDB;`

	// execute
	if _, err := db.Exec(productTable); err != nil {
		log.Fatal("Failed to create product table:", err)
	} else {
		log.Println("product table created or already exists")
	}

	if _, err := db.Exec(shoppingCartTable); err != nil {
		log.Fatal("Failed to create shopping_cart table:", err)
	} else {
		log.Println("shopping_cart table created or already exists")
	}

	if _, err := db.Exec(cartItemTable); err != nil {
		log.Fatal("Failed to create cart_item table:", err)
	} else {
		log.Println("cart_item table created or already exists")
	}

	if _, err := db.Exec(inventoryTable); err != nil {
		log.Fatal("Failed to create inventory table:", err)
	} else {
		log.Println("inventory table created or already exists")
	}
}

/* Internal function: Generate Product data and store in products */
func generateData(number int) {
	// define sample arrays
	brand_samples := []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon", "Zeta"}
	category_samples := []string{"Electronics", "Books", "Home", "Food", "Toy", "Office Supplies", "Health", "Personal Care"}

	// define size of samples
	brand_samples_size := len(brand_samples)
	category_samples_size := len(category_samples)

	// initialize a slice to store Product pointers
	productPtrs := make([]*Product, 0, number)

	// generate and store product data
	for i := 1; i <= number; i++ {
		brand := brand_samples[(i-1)%brand_samples_size]
		category := category_samples[(i-1)%category_samples_size]
		name := fmt.Sprintf("Product %s %d", brand, i)
		p := &Product{
			// ID:            int32(i),
			Name:          name,
			Category:      category,
			Description:   "",
			Brand:         brand,
			NameLower:     strings.ToLower(name),
			CategoryLower: strings.ToLower(category),
		}
		productPtrs = append(productPtrs, p)
	}

	// store in the database
	if err := saveProductsBatch(productPtrs, 100); err != nil {
		log.Fatal("Failed to batch insert products:", err)
	}
	log.Println("Successfully inserted", number, "products into database")

}

/* Internal function: Store created product data in the database in batch */
func saveProductsBatch(products []*Product, batchSize int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		} else if err != nil {
			rbErr := tx.Rollback()
			if rbErr != nil {
				log.Printf("transaction rollback failed: %v", rbErr)
			}
		}
	}()

	// handle in batch
	for i := 0; i < len(products); i += batchSize {
		end := min(i+batchSize, len(products))
		batch := products[i:end]

		// Joint VALUES (?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?) ...
		placeholders := make([]string, 0, len(batch))
		values := make([]interface{}, 0, len(batch)*6)

		for _, p := range batch {
			placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?)")
			values = append(values,
				p.Name, p.Category, p.Brand, p.Description, p.NameLower, p.CategoryLower,
			)
		}

		query := fmt.Sprintf(`
            INSERT INTO product (name, category, brand, description, name_lowercase, category_lowercase)
            VALUES %s
        `, strings.Join(placeholders, ","))

		if _, err = tx.Exec(query, values...); err != nil {
			return err // err != nil -> defer rollback automatically
		}
	}

	return tx.Commit()
}

/* Internal function: Query Prduct in the database by product_id */
func queryProductByID(productID int32) (Product, error) {
	query := `
		SELECT product_id, name, category, brand, description, name_lowercase, category_lowercase
		FROM product
		WHERE product_id = ?
	`
	var p Product
	err := db.QueryRow(query, productID).Scan(
		&p.ID,
		&p.Name,
		&p.Category,
		&p.Brand,
		&p.Description,
		&p.NameLower,
		&p.CategoryLower,
	)
	return p, err
}

/* Internal function: Update production information in the database if the product exists*/
func updateProductIfExists(p Product) error {
	// Start a transaction
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // Rollback once fail

	// Check if the record exists
	var exists int
	err = tx.QueryRow("SELECT 1 FROM product WHERE product_id = ? FOR UPDATE", p.ID).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("product with ID %d does not exist", p.ID)
		}
		return err
	}

	// Execute update
	query := `
		UPDATE product
		SET name = ?, category = ?, brand = ?, description = ?, name_lowercase = ?, category_lowercase = ?
		WHERE product_id = ?
	`
	_, err = tx.Exec(query, p.Name, p.Category, p.Brand, p.Description, p.NameLower, p.CategoryLower, p.ID)
	if err != nil {
		return err
	}

	// Commit
	return tx.Commit()
}

/* Clear data in shopping_carts and cart_itmes tables in the database (keep tables, and product data) */
func clearCartsData(c *gin.Context) {
	tables := []string{"cart_item", "shopping_cart"}

	for _, table := range tables {
		_, err := db.Exec(fmt.Sprintf("DELETE FROM %s", table))
		if err != nil {
			err500 := ErrorResponse{
				Err:     "DB_ERROR",
				Message: "Fail to delete data in shopping_carts and cart_itmes tables",
				Details: err.Error(),
			}
			c.JSON(http.StatusInternalServerError, err500)
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"message": "Shopping cart data cleared"})
}
