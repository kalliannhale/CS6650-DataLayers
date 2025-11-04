import requests
import time
import json
import datetime
import os

# ---
# CONFIGURATION
# ---
# Set your ALB DNS name as an environment variable before running
# Example: export ALB_URL="http://your-alb-dns-name.elb.amazonaws.com"
APP_ADDRESS = os.environ.get("ALB_URL")
if not APP_ADDRESS:
    print("Error: ALB_URL environment variable not set.")
    print("Please set it, e.g.: export ALB_URL=\"http://your-alb-dns.com\"")
    exit(1)

results = []
# NOTE: The ALB listens on port 80, so we don't add :8080
base_url = APP_ADDRESS
OUTPUT_FILE = "dynamodb_test_results.json"

# ---
# Operation Functions (Identical to your teammate's)
# ---

# POST /shopping-carts
def create_carts():
    print("Running: 50 CREATE_CART operations...")
    cart_ids = []
    for i in range(50):
        start_time = time.time()
        try:
            r = requests.post(f"{base_url}/shopping-carts", json={"customer_id": i + 1})
            end_time = time.time()
            
            if r.status_code == 201:
                # Get the cart_id. Python doesn't care if it's a
                # string (from DynamoDB) or int (from MySQL).
                cart_id = r.json().get("cart_id")
                if cart_id is not None:
                    cart_ids.append(cart_id)
            
            log_result("create_cart", start_time, end_time, r)

        except requests.RequestException as e:
            end_time = time.time()
            # Create a mock response object for logging failures
            log_result("create_cart", start_time, end_time, 
                       type("MockResponse", (object,), {"status_code": 503, "text": str(e)})())

    return cart_ids

# POST /shopping-carts/{id}/items
def add_items(cart_ids):
    print("Running: 50 ADD_ITEMS operations...")
    if not cart_ids:
        print("  No cart IDs to test. Skipping add_items.")
        return

    # Run 50 'add' operations, cycling through the cart IDs
    for i in range(50):
        cart_id = cart_ids[i % len(cart_ids)]
        
        # This payload matches the API: {"items": [...]}
        payload = {
            "items": [
                {"product_id": pid, "quantity": q} for pid, q in zip(range(101, 106), range(1, 6))
            ]
        }
        
        start_time = time.time()
        try:
            r = requests.post(f"{base_url}/shopping-carts/{cart_id}/items", json=payload)
            end_time = time.time()
            log_result("add_items", start_time, end_time, r)

        except requests.RequestException as e:
            end_time = time.time()
            log_result("add_items", start_time, end_time, 
                       type("MockResponse", (object,), {"status_code": 503, "text": str(e)})())

# GET /shopping-carts/{id} 
def get_carts(cart_ids):
    print("Running: 50 GET_CART operations...")
    if not cart_ids:
        print("  No cart IDs to test. Skipping get_carts.")
        return

    # Run 50 'get' operations, cycling through the cart IDs
    for i in range(50):
        cart_id = cart_ids[i % len(cart_ids)]
        
        start_time = time.time()
        try:
            r = requests.get(f"{base_url}/shopping-carts/{cart_id}")
            end_time = time.time()
            log_result("get_cart", start_time, end_time, r)
        
        except requests.RequestException as e:
            end_time = time.time()
            log_result("get_cart", start_time, end_time, 
                       type("MockResponse", (object,), {"status_code": 503, "text": str(e)})())

# ---
# Helper Function
# ---

def log_result(op, start_time, end_time, response):
    """Helper to format the log as required by the assignment"""
    entry = {
        "operation": op,
        "response_time": (end_time - start_time) * 1000, # in milliseconds
        "success": 200 <= response.status_code < 300,
        "status_code": response.status_code,
        "timestamp": datetime.datetime.now(datetime.timezone.utc).isoformat()
    }
    results.append(entry)
    if not entry["success"]:
        print(f"  > FAILED {op} (Status {entry['status_code']}): {response.text[:100]}...")

# ---
# Run tests
# ---
def main():
    print(f"Starting test suite on {base_url}...")
    
    # NOTE: Your teammate's /debug/clear-carts is not included.
    # For a clean test, run 'terraform destroy' and 'terraform apply'
    # to get a fresh, empty DynamoDB table.

    cart_ids = create_carts()
    
    # Check if create_carts actually succeeded before continuing
    if not cart_ids:
        print("No carts were created. Aborting test.")
        return

    add_items(cart_ids)
    get_carts(cart_ids)

    # save results
    with open(OUTPUT_FILE, "w") as f:
        json.dump(results, f, indent=2)

    print(f"\nTest complete. {len(results)} operations logged.")
    print(f"Results saved to {OUTPUT_FILE}")

if __name__ == "__main__":
    main()
