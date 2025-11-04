import requests, time, json, datetime

APP_ADDRESS = "<ALB_DNS_NEEDS_TO_REPLACE>"

results = []
base_url =f"http://{APP_ADDRESS}:8080" 

# POST /shopping-carts
def create_carts():
    cart_ids = []
    for i in range(50):
        start = time.time()
        r = requests.post(f"{base_url}/shopping-carts", json={"customer_id": i + 1})
        end = time.time()

        cart_id = r.json().get("cart_id")
        if cart_id is not None:
            cart_ids.append(cart_id)

        results.append({
            "operation": "create_cart",
            "response_time": (end - start) * 1000,
            "success": r.status_code == 201,
            "status_code": r.status_code,
            "timestamp": datetime.datetime.now(datetime.timezone.utc).isoformat()
        })
    return cart_ids

# POST /shopping-carts/{id}/items
def add_items(cart_ids):
    for cart_id in cart_ids:
        # example payload with 1–10 items, product_ids 101–105
        payload = {
            "items": [{"product_id": pid, "quantity": q} for pid, q in zip(range(101, 106), range(1, 11))]
        }
        start = time.time()
        r = requests.post(f"{base_url}/shopping-carts/{cart_id}/items", json=payload)
        end = time.time()

        results.append({
            "operation": "add_items",
            "response_time": (end - start) * 1000,
            "success": r.status_code == 200,
            "status_code": r.status_code,
            "timestamp": datetime.datetime.now(datetime.timezone.utc).isoformat()
        })

# GET /shopping-carts/{id} 
def get_carts(cart_ids):
    for cart_id in cart_ids:
        start = time.time()
        r = requests.get(f"{base_url}/shopping-carts/{cart_id}")
        end = time.time()

        results.append({
            "operation": "get_cart",
            "response_time": (end - start) * 1000,
            "success": r.status_code == 200,
            "status_code": r.status_code,
            "timestamp": datetime.datetime.now(datetime.timezone.utc).isoformat()
        })

# Run tests
def main():
    # clear created data for repeatable test
    requests.delete(f"{base_url}/debug/clear-carts")

    print("Creating carts...")
    cart_ids = create_carts()
    print("Adding items...")
    add_items(cart_ids)
    print("Getting carts...")
    get_carts(cart_ids)

    # save results
    with open("mysql_test_results.json", "w") as f:
        json.dump(results, f, indent=2)

    print("Test complete. Results saved to mysql_test_results.json")

if __name__ == "__main__":
    main()