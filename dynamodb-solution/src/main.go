package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// database client
var dbClient *dynamodb.Client
var tableName string

// ---
// Structs (Matching your teammate's API)
// ---

// ErrorResponse struct
type ErrorResponse struct {
	Err     string `json:"error"`
	Message string `json:"message"`
	Details string `json:"details"`
}

// Structs for GET /shopping-carts/:id
// We must match the teammate's API response
type cartItemInfo struct {
	ProductID int32 `json:"product_id"`
	ProductName string `json:"product_name"`
	Quantity uint `json:"quantity"`
}
type shoppingCartInfo struct {
	CartID     string         `json:"cart_id"` 
	CustomerID uint64         `json:"customer_id"`
	Status     string         `json:"status"`
	Items      []cartItemInfo `json:"items"`
}

// Structs for POST /shopping-carts/:id/items
type updateCartItem struct {
	ProductID int32 `json:"product_id" binding:"required"`
	Quantity  uint  `json:"quantity"`
}
type updateCartItemsRequest struct {
	Items []updateCartItem `json:"items" binding:"required"`
}

// ---
// DynamoDB Internal Structs (for mapping)
// ---
type cartMetadata struct {
	PK         string `dynamodbav:"PK"`
	SK         string `dynamodbav:"SK"`
	GSI1PK     string `dynamodbav:"GSI1PK"`
	GSI1SK     string `dynamodbav:"GSI1SK"`
	CartID     string `dynamodbav:"cart_id"`
	CustomerID uint64 `dynamodbav:"customer_id"`
	Status     string `dynamodbav:"status"`
}
type cartItemData struct {
	PK        string `dynamodbav:"PK"`
	SK        string `dynamodbav:"SK"`
	ProductID int32  `dynamodbav:"product_id"`
	Quantity  uint   `dynamodbav:"quantity"`
	ProductName string `dynamodbav:"product_name"`
}

func main() {
	// Initialize database
	InitDB()

	// Router (Matching your teammate)
	router := gin.Default()

	// Shopping cart service endpoints
	router.POST("/shopping-carts", createShoppingCart)
	router.GET("/shopping-carts/:id", getShoppingCart)
	router.POST("/shopping-carts/:id/items", updateItemToShoppingCart)

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	log.Println("Starting server on :8080")
	router.Run(":8080")
}

/*
Internal function: Initialize DynamoDB client
*/
func InitDB() {
	// Get table name from environment
	tableName = os.Getenv("DYNAMODB_TABLE_NAME")
	if tableName == "" {
		log.Fatal("DYNAMODB_TABLE_NAME environment variable not set")
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Unable to load AWS config: %v", err)
	}

	dbClient = dynamodb.NewFromConfig(cfg)
	log.Printf("Successfully connected to DynamoDB. Using table: %s", tableName)
}

// ---
// API Handlers (DynamoDB Version)
// ---

/*
POST /shopping-carts
Creates a new shopping cart.
*/
func createShoppingCart(c *gin.Context) {
	var req struct {
		CustomerID uint64 `json:"customer_id"`
	}

	// 1. Parse request (Identical to teammate)
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Err:     "INVALID_INPUT",
			Message: "The provided request body is invalid",
			Details: err.Error(),
		})
		return
	}
	if req.CustomerID == 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Err:     "INVALID_INPUT",
			Message: "customer_id cannot be 0",
		})
		return
	}

	// 2. Create Cart in DynamoDB
	cartID := uuid.New().String() // Our Cart ID is a string
	cartPK := fmt.Sprintf("CART#%s", cartID)
	custPK := fmt.Sprintf("CUST#%d", req.CustomerID)

	meta := cartMetadata{
		PK:         cartPK,
		SK:         "CART",
		GSI1PK:     custPK,
		GSI1SK:     cartPK,
		CartID:     cartID,
		CustomerID: req.CustomerID,
		Status:     "active", // Default to active, just like teammate
	}

	dbItem, err := attributevalue.MarshalMap(meta)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Err:     "INTERNAL_ERROR",
			Message: "Failed to marshal cart data",
			Details: err.Error(),
		})
		return
	}

	// We use "PutItem" to create the cart's metadata row
	_, err = dbClient.PutItem(c.Request.Context(), &dynamodb.PutItemInput{
		TableName:           aws.String(tableName),
		Item:                dbItem,
		ConditionExpression: aws.String("attribute_not_exists(PK)"), // Fail if cart ID already exists
	})

	if err != nil {
		// Handle errors
		var condFailed *types.ConditionalCheckFailedException
		if errors.As(err, &condFailed) {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Err:     "INTERNAL_ERROR",
				Message: "Cart ID collision, try again.",
				Details: err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Err:     "DB_ERROR",
				Message: "Failed to create shopping cart",
				Details: err.Error(),
			})
		}
		return
	}

	// 3. Send Response (Matches teammate's format, but our ID is a string)
	c.JSON(http.StatusCreated, gin.H{
		"cart_id": cartID,
		"status":  "active",
	})
}

/*
GET /shopping-carts/:id
Get a shopping cart with its items
*/
func getShoppingCart(c *gin.Context) {
	// 1. Parse cart ID (c.Param is always a string)
	cartIDStr := c.Param("id")

	// 2. Query DynamoDB for all items in the cart
	cartPK := fmt.Sprintf("CART#%s", cartIDStr)
	output, err := dbClient.Query(c.Request.Context(), &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: cartPK},
		},
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Err:     "DB_ERROR",
			Message: "Failed to query cart",
			Details: err.Error(),
		})
		return
	}

	// This is how we check for "Not Found" in DynamoDB
	if len(output.Items) == 0 {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Err:     "CART_NOT_FOUND",
			Message: "Shopping cart not found",
			Details: fmt.Sprintf("Shopping cart %s does not exist", cartIDStr),
		})
		return
	}

	// 3. Build the response (using teammate's 'shoppingCartInfo' struct)
	response := shoppingCartInfo{Items: []cartItemInfo{}}
	foundCartMeta := false

	// Loop through all rows (cart metadata + item data) returned by the query
	for _, dbItem := range output.Items {
		// Use a type assertion to get the Sort Key (SK) value
		skValue, ok := dbItem["SK"].(*types.AttributeValueMemberS)
		if !ok {
			continue // Skip item if SK is not a string
		}

		if skValue.Value == "CART" {
			// This is the main cart metadata row
			var meta cartMetadata
			attributevalue.UnmarshalMap(dbItem, &meta)
			response.CartID = meta.CartID
			response.CustomerID = meta.CustomerID
			response.Status = meta.Status
			foundCartMeta = true
		} else if strings.HasPrefix(skValue.Value, "ITEM#") {
			// This is an item row
			var item cartItemData
			attributevalue.UnmarshalMap(dbItem, &item)
			response.Items = append(response.Items, cartItemInfo{
				ProductID: item.ProductID,
				ProductName: item.ProductName,
				Quantity:  item.Quantity,
			})
		}
	}

	if !foundCartMeta {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Err:     "CART_NOT_FOUND",
			Message: "Cart data corrupted, metadata not found",
			Details: fmt.Sprintf("Cart %s has items but no main record", cartIDStr),
		})
		return
	}

	// 4. Send Response
	c.JSON(http.StatusOK, response)
}

func lookupProductName(productID int32) string {
    return fmt.Sprintf("Widget #%d", productID)
}

/*
POST /shopping-carts/:id/items
Add or update items in existing cart
*/
func updateItemToShoppingCart(c *gin.Context) {
	// 1. Parse Cart ID
	cartIDStr := c.Param("id")

	// 2. Parse Request Body (Identical to teammate)
	var req updateCartItemsRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Err:     "INVALID_INPUT",
			Message: "The provided request body is invalid",
			Details: err.Error(),
		})
		return
	}

	// 3. Write to DynamoDB using BatchWriteItem
	// This is the NoSQL equivalent of your teammate's "INSERT...ON DUPLICATE KEY UPDATE"
	cartPK := fmt.Sprintf("CART#%s", cartIDStr)
	writeRequests := []types.WriteRequest{}

	// Check for duplicates in the request, just like your teammate
	seen := make(map[int32]bool)
	for _, item := range req.Items {
		if seen[item.ProductID] {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Err:     "INVALID_INPUT",
				Message: "Duplicate product_ids in request",
			})
			return
		}
		seen[item.ProductID] = true

		// DynamoDB "Put" will create or overwrite, just like
		// your teammate's "ON DUPLICATE KEY UPDATE".
		// We will assume quantity > 0 for this.
		if item.Quantity > 0 {
			productName := lookupProductName(item.ProductID)
			itemSK := fmt.Sprintf("ITEM#%d", item.ProductID)
			dbItem := cartItemData{
				PK:        cartPK,
				SK:        itemSK,
				ProductID: item.ProductID,
				Quantity:  item.Quantity,
				ProductName: productName,
			}

			marshalledItem, err := attributevalue.MarshalMap(dbItem)
			if err != nil {
				c.JSON(http.StatusInternalServerError, ErrorResponse{Err: "INTERNAL_ERROR", Message: "Failed to marshal item", Details: err.Error()})
				return
			}

			writeRequests = append(writeRequests, types.WriteRequest{
				PutRequest: &types.PutRequest{Item: marshalledItem},
			})
		}
		// Note: We're not handling deletes (quantity=0), but neither is your teammate.
	}

	// Check if there's anything to write
	if len(writeRequests) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "No items to update."})
		return
	}

	// Call BatchWriteItem
	_, err := dbClient.BatchWriteItem(c.Request.Context(), &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			tableName: writeRequests,
		},
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Err:     "DB_ERROR",
			Message: "Failed to batch write items",
			Details: err.Error(),
		})
		return
	}

	// 4. Send Response (Identical to teammate)
	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Cart %s updated", cartIDStr),
	})
}