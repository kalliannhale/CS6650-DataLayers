import requests
import time
import json
import os

ALB_URL = os.environ.get("ALB_URL", "http://CS6650L2-alb-671481294.us-east-1.elb.amazonaws.com")
API_URL = f"{ALB_URL}/shopping-carts"

results = []

def log_result(op, start_time, response):
    elapsed_ms = (time.time() - start_time) * 1000
    success = response.status_code in [200, 201]
    
    log_entry = {
        "operation": op,
        "response_time": elapsed_ms,
        "success": success,
        "status_code": response.status_code,
        "timestamp": time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime()),
    }
    results.append(log_entry)
    print(f"  {op}: {response.status_code} ({elapsed_ms:.2f} ms)")

def run_test():
    print(f"Starting 150-operation test on {API_URL}...")
    cart_ids = []

    # 1. 50 Create Cart Operations
    print("Phase 1: 50 CREATE_CART operations")
    for i in range(50):
        start = time.time()
        # This payload matches the API: {"customer_id": uint64}
        payload = {"customer_id": 1000 + i} 
        try:
            r = requests.post(API_URL, json=payload)
            log_result("create_cart", start, r)
            if r.status_code == 201:
                # Get the cart_id. Python doesn't care if it's a
                # string (from DynamoDB) or int (from MySQL).
                cart_ids.append(r.json().get("cart_id"))
        except requests.RequestException as e:
            log_result("create_cart", start, e.response or type("R", (object,), {"status_code": 503}))

    # 2. 50 Add Items Operations
    print(f"\nPhase 2: 50 ADD_ITEMS operations (on {len(cart_ids)} carts)")
    for i in range(50):
        if not cart_ids:
            print("  No carts to add to, skipping...")
            continue
            
        cart_id = cart_ids[i % len(cart_ids)] # Cycle through created carts
        item_url = f"{API_URL}/{cart_id}/items"
        
        # This payload matches the API: {"items": [...]}
        payload = {
            "items": [
                {"product_id": 101, "quantity": 1},
                {"product_id": 102, "quantity": 2}
            ]
        }
        
        start = time.time()
        try:
            r = requests.post(item_url, json=payload)
            log_result("add_items", start, r)
        except requests.RequestException as e:
            log_result("add_items", start, e.response or type("R", (object,), {"status_code": 503}))

    # 3. 50 Get Cart Operations
    print(f"\nPhase 3: 50 GET_CART operations (on {len(cart_ids)} carts)")
    for i in range(50):
        if not cart_ids:
            print("  No carts to get, skipping...")
            continue
            
        cart_id = cart_ids[i % len(cart_ids)]
        cart_url = f"{API_URL}/{cart_id}"
        
        start = time.time()
        try:
            r = requests.get(cart_url)
            log_result("get_cart", start, r)
        except requests.RequestException as e:
            log_result("get_cart", start, e.response or type("R", (object,), {"status_code": 503}))

    # Save results to the required file
    output_file = "dynamodb_test_results.json"
    with open(output_file, "w") as f:
        json.dump(results, f, indent=2)
    
    print(f"\nTest complete. Results saved to {output_file}")

if __name__ == "__main__":
    run_test()