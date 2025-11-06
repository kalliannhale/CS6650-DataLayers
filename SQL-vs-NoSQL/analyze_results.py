import json
import numpy as np
from tabulate import tabulate

def load_combined_results():
    """Load the combined results file"""
    try:
        with open('combined_results.json', 'r') as f:
            data = json.load(f)
        print("✓ Loaded combined_results.json\n")
        return data
    except FileNotFoundError:
        print("❌ Error: combined_results.json not found")
        return None

def inspect_data_structure(db_data, db_name):
    """Inspect the actual structure of the data"""
    print(f"\n--- Inspecting {db_name} data structure ---")
    if db_data:
        print(f"Total records: {len(db_data)}")
        print(f"Sample record: {db_data[0]}")
        
        # Get unique operation names
        operations = set(item['operation'] for item in db_data)
        print(f"Unique operations found: {operations}")
    print()

def calculate_percentile(data, percentile):
    """Calculate percentile for response times"""
    return np.percentile(data, percentile)

def calculate_metrics(db_data):
    """Calculate all metrics for a database"""
    response_times = [item['response_time'] for item in db_data]
    successes = [item.get('success', True) for item in db_data]
    
    metrics = {
        'avg': np.mean(response_times),
        'p50': calculate_percentile(response_times, 50),
        'p95': calculate_percentile(response_times, 95),
        'p99': calculate_percentile(response_times, 99),
        'success_rate': (sum(successes) / len(successes)) * 100,
        'total_ops': len(db_data)
    }
    
    return metrics

def calculate_operation_metrics(db_data):
    """Calculate average response time per operation type"""
    operations = {}
    
    for item in db_data:
        op = item['operation']
        if op not in operations:
            operations[op] = []
        operations[op].append(item['response_time'])
    
    op_metrics = {}
    for op, times in operations.items():
        op_metrics[op] = np.mean(times)
        print(f"  {op}: {len(times)} records, avg = {op_metrics[op]:.2f} ms")
    
    return op_metrics

def normalize_operation_name(op_name):
    """Normalize operation names to standard format"""
    op_lower = op_name.lower()
    
    # Handle various possible naming conventions
    if 'create' in op_lower:
        return 'CREATE_CART'
    elif 'add' in op_lower:
        return 'ADD_ITEMS'
    elif 'get' in op_lower or 'retrieve' in op_lower or 'read' in op_lower:
        return 'GET_CART'
    else:
        return op_name.upper()

def format_comparison_table(mysql_metrics, dynamodb_metrics):
    """Format the main comparison table"""
    
    metrics = [
        ('Avg Response Time (ms)', 'avg'),
        ('P50 Response Time (ms)', 'p50'),
        ('P95 Response Time (ms)', 'p95'),
        ('P99 Response Time (ms)', 'p99'),
        ('Success Rate (%)', 'success_rate'),
        ('Total Operations', 'total_ops')
    ]
    
    table_data = []
    
    for metric_name, metric_key in metrics:
        mysql_val = mysql_metrics[metric_key]
        dynamodb_val = dynamodb_metrics[metric_key]
        
        # Determine winner and margin
        if metric_key == 'success_rate':
            winner = 'MySQL' if mysql_val > dynamodb_val else 'DynamoDB'
            margin = f"{abs(mysql_val - dynamodb_val):.2f}%"
        elif metric_key == 'total_ops':
            winner = 'Tie'
            margin = 'N/A'
        else:
            winner = 'MySQL' if mysql_val < dynamodb_val else 'DynamoDB'
            margin = f"{abs(mysql_val - dynamodb_val):.2f} ms"
        
        # Format values
        if metric_key == 'success_rate':
            mysql_str = f"{mysql_val:.2f}"
            dynamodb_str = f"{dynamodb_val:.2f}"
        elif metric_key == 'total_ops':
            mysql_str = f"{int(mysql_val)}"
            dynamodb_str = f"{int(dynamodb_val)}"
        else:
            mysql_str = f"{mysql_val:.2f}"
            dynamodb_str = f"{dynamodb_val:.2f}"
        
        table_data.append([metric_name, mysql_str, dynamodb_str, winner, margin])
    
    return table_data

def format_operation_table(mysql_ops, dynamodb_ops):
    """Format the operation-specific breakdown table"""
    
    # Normalize all operation names
    mysql_normalized = {}
    for op, val in mysql_ops.items():
        normalized = normalize_operation_name(op)
        mysql_normalized[normalized] = val
    
    dynamodb_normalized = {}
    for op, val in dynamodb_ops.items():
        normalized = normalize_operation_name(op)
        dynamodb_normalized[normalized] = val
    
    # Get all unique operation types
    all_ops = set(list(mysql_normalized.keys()) + list(dynamodb_normalized.keys()))
    
    table_data = []
    
    for op_name in sorted(all_ops):
        mysql_avg = mysql_normalized.get(op_name, 0)
        dynamodb_avg = dynamodb_normalized.get(op_name, 0)
        
        if mysql_avg == 0 or dynamodb_avg == 0:
            faster = "Data missing"
        elif mysql_avg < dynamodb_avg:
            faster = f"MySQL by {dynamodb_avg - mysql_avg:.2f} ms"
        else:
            faster = f"DynamoDB by {mysql_avg - dynamodb_avg:.2f} ms"
        
        table_data.append([
            op_name,
            f"{mysql_avg:.2f}",
            f"{dynamodb_avg:.2f}",
            faster
        ])
    
    return table_data

def main():
    # Load combined results
    combined = load_combined_results()
    if not combined:
        return
    
    # Extract data
    mysql_data = combined['mysql']
    dynamodb_data = combined['dynamodb']
    
    # Inspect data structure
    inspect_data_structure(mysql_data, "MySQL")
    inspect_data_structure(dynamodb_data, "DynamoDB")
    
    # Calculate metrics
    print("Calculating MySQL metrics...")
    mysql_metrics = calculate_metrics(mysql_data)
    mysql_ops = calculate_operation_metrics(mysql_data)
    
    print("\nCalculating DynamoDB metrics...")
    dynamodb_metrics = calculate_metrics(dynamodb_data)
    dynamodb_ops = calculate_operation_metrics(dynamodb_data)
    
    # Print Comparison Table
    print("\n" + "=" * 80)
    print("REQUIRED COMPARISON TABLE")
    print("=" * 80)
    comparison_table = format_comparison_table(mysql_metrics, dynamodb_metrics)
    print(tabulate(comparison_table, 
                   headers=['Metric', 'MySQL', 'DynamoDB', 'Winner', 'Margin'],
                   tablefmt='grid'))
    print("\nData Source: combined_results.json")
    
    # Print Operation-Specific Breakdown
    print("\n" + "=" * 80)
    print("OPERATION-SPECIFIC BREAKDOWN")
    print("=" * 80)
    operation_table = format_operation_table(mysql_ops, dynamodb_ops)
    print(tabulate(operation_table,
                   headers=['Operation', 'MySQL Avg (ms)', 'DynamoDB Avg (ms)', 'Faster By'],
                   tablefmt='grid'))
    
    # Save to text file for easy copying
    with open('comparison_tables.txt', 'w') as f:
        f.write("=" * 80 + "\n")
        f.write("REQUIRED COMPARISON TABLE\n")
        f.write("=" * 80 + "\n")
        f.write(tabulate(comparison_table, 
                        headers=['Metric', 'MySQL', 'DynamoDB', 'Winner', 'Margin'],
                        tablefmt='grid'))
        f.write("\n\nData Source: combined_results.json\n")
        
        f.write("\n" + "=" * 80 + "\n")
        f.write("OPERATION-SPECIFIC BREAKDOWN\n")
        f.write("=" * 80 + "\n")
        f.write(tabulate(operation_table,
                        headers=['Operation', 'MySQL Avg (ms)', 'DynamoDB Avg (ms)', 'Faster By'],
                        tablefmt='grid'))
    
    print("\n✓ Results saved to comparison_tables.txt")
    
    # Export to JSON for further use
    results = {
        'comparison_metrics': {
            'mysql': mysql_metrics,
            'dynamodb': dynamodb_metrics
        },
        'operation_breakdown': {
            'mysql': mysql_ops,
            'dynamodb': dynamodb_ops
        }
    }
    
    with open('analysis_metrics.json', 'w') as f:
        json.dump(results, f, indent=2)
    
    print("✓ Metrics saved to analysis_metrics.json")

if __name__ == "__main__":
    main()