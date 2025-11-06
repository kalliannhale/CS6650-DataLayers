import json
from collections import Counter

def verify_operations(data, db_name):
    """Verify that the data contains exactly 150 operations (50 of each 
type)"""
    operations = [item['operation'] for item in data]
    op_counts = Counter(operations)
    
    print(f"\n{db_name} - Operation counts:")
    for op, count in op_counts.items():
        print(f"  {op}: {count}")
    
    # Verify counts
    expected = {'create': 50, 'add': 50, 'get': 50}
    is_valid = all(op_counts.get(op, 0) == count for op, count in 
expected.items())
    
    total = len(data)
    print(f"  Total: {total}")
    
    if total != 150:
        print(f"  ⚠️  WARNING: {db_name} does not have the expected operation distribution!")
        return False
    else:
        print(f"  ✓ {db_name} verified successfully")
        return True
    
    return is_valid and total == 150

def combine_results():
    """Combine MySQL and DynamoDB results into a single file"""
    
    # Load both files
    try:
        with open('mysql_test_results.json', 'r') as f:
            mysql_data = json.load(f)
        print("✓ Loaded mysql_test_results.json")
    except FileNotFoundError:
        print("❌ Error: mysql_test_results.json not found")
        return
    
    try:
        with open('dynamodb_test_results.json', 'r') as f:
            dynamodb_data = json.load(f)
        print("✓ Loaded dynamodb_test_results.json")
    except FileNotFoundError:
        print("❌ Error: dynamodb_test_results.json not found")
        return
    
    # Verify both datasets
    mysql_valid = verify_operations(mysql_data, "MySQL")
    dynamodb_valid = verify_operations(dynamodb_data, "DynamoDB")
    
    if not (mysql_valid and dynamodb_valid):
        print("\n❌ Data verification failed. Please check your source files.")
        return
    
    # Combine the results
    combined = {
        "mysql": mysql_data,
        "dynamodb": dynamodb_data,
        "metadata": {
            "total_operations": len(mysql_data) + len(dynamodb_data),
            "operations_per_db": 150,
            "databases": ["mysql", "dynamodb"],
            "operation_types": ["create", "add", "get"],
            "operations_per_type": 50
        }
    }
    
    # Save combined results
    with open('combined_results.json', 'w') as f:
        json.dump(combined, f, indent=2)
    
    print("\n✓ Successfully created combined_results.json")
    print(f"  Total records: {combined['metadata']['total_operations']}")

if __name__ == "__main__":
    combine_results()