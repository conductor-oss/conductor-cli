# JavaScript Worker Capabilities and Dependencies

⚠️ **EXPERIMENTAL FEATURE** - This feature is experimental and may change in future releases.

## Current Implementation

The JavaScript worker uses **Goja**, a pure Go implementation of ECMAScript 5.1+. This provides excellent performance and security, but has different capabilities compared to Node.js.

## What You Can Do

### ✅ Available Features

#### 1. Standard JavaScript (ES5+)
- Variables, functions, objects, arrays
- String manipulation
- Number operations
- Boolean logic
- Regular expressions
- Date and time operations
- JSON parsing and stringification
- Control flow (if/else, loops, switch, try/catch)
- Array methods (map, filter, reduce, forEach, etc.)
- Object methods

```javascript
// All standard JavaScript works
const data = $.task.inputData;
const result = data.items
  .filter(item => item.price > 100)
  .map(item => ({ ...item, discount: item.price * 0.1 }));
```

#### 2. Built-in Utilities

The worker provides several utility namespaces with Go-powered functions:

##### HTTP Requests (`http`)
```javascript
// GET request
const response = http.get("https://api.example.com/data", {
  "Authorization": "Bearer token123",
  "Content-Type": "application/json"
});

if (response.error) {
  console.log("HTTP error:", response.error);
} else {
  console.log("Status:", response.status);
  console.log("Body:", response.body);  // Parsed JSON
  console.log("Text:", response.text);  // Raw text
}

// POST request
const postResponse = http.post(
  "https://api.example.com/create",
  { "Content-Type": "application/json" },
  JSON.stringify({ name: "test", value: 123 })
);

// PUT request
const putResponse = http.put(
  "https://api.example.com/update/123",
  { "Authorization": "Bearer token" },
  JSON.stringify({ status: "active" })
);

// DELETE request
const deleteResponse = http.delete(
  "https://api.example.com/delete/123",
  { "Authorization": "Bearer token" }
);
```

##### Cryptographic Functions (`crypto`)
```javascript
// Hash functions
const md5Hash = crypto.md5("hello world");
const sha1Hash = crypto.sha1("hello world");
const sha256Hash = crypto.sha256("hello world");

// Base64 encoding/decoding
const encoded = crypto.base64Encode("Hello World");
const decoded = crypto.base64Decode(encoded);

// Example: Generate API signature
const apiKey = "my-secret-key";
const timestamp = Date.now();
const signature = crypto.sha256(apiKey + timestamp);
```

##### Utility Functions (`util`)
```javascript
// Sleep/delay execution
util.sleep(1000); // Sleep for 1 second (1000ms)

// Generate unique ID
const uniqueId = util.uuid();

// Access environment variables
const apiKey = util.env("API_KEY");
const dbHost = util.env("DATABASE_HOST");
```

##### String Utilities (`str`)
```javascript
// String manipulation
const upper = str.toUpper("hello");        // "HELLO"
const lower = str.toLower("WORLD");        // "world"
const trimmed = str.trim("  spaces  ");    // "spaces"

// String operations
const parts = str.split("a,b,c", ",");     // ["a", "b", "c"]
const joined = str.join(["a", "b"], "-");  // "a-b"
const replaced = str.replace("hello world", "world", "goja"); // "hello goja"

// String testing
const hasIt = str.contains("hello world", "world");    // true
const starts = str.hasPrefix("hello", "hel");          // true
const ends = str.hasSuffix("world", "ld");             // true
```

#### 3. Access to Task Data
```javascript
// Full access to task information
const taskId = $.task.taskId;
const workflowId = $.task.workflowInstanceId;
const input = $.task.inputData;
const retryCount = $.task.retryCount;
```

#### 4. Console Logging
```javascript
console.log("Processing task:", $.task.taskId);
console.error("Error occurred");
```

### ❌ NOT Available

#### Node.js APIs
- No `require()` or `import`
- No `fs` (filesystem) module
- No `process` module
- No `Buffer` class
- No Node.js built-in modules

#### NPM Packages
- Cannot install or use npm packages directly
- No package.json support
- No node_modules

#### Modern JavaScript Features
- No async/await (ES2017)
- No Promises (limited support)
- No ES6 modules (import/export)
- No arrow functions in some contexts

#### System Access
- No direct file system access
- No subprocess execution
- No network sockets (except via http utilities)

## How to Add Dependencies

### Option 1: Use Built-in Utilities (Recommended)

The worker provides HTTP, crypto, and string utilities. Use these for most needs:

```javascript
(function() {
  // Instead of axios or fetch
  var response = http.get("https://api.example.com/data", {
    "Authorization": "Bearer " + util.env("API_TOKEN")
  });

  // Instead of crypto library
  var hash = crypto.sha256(JSON.stringify($.task.inputData));

  // Instead of lodash string functions
  var cleanedData = str.trim(str.toLower(input.username));

  // ... rest of your code
})();
```

### Option 2: Inline JavaScript Libraries

Copy small JavaScript libraries directly into your worker file:

```javascript
// worker.js
// Include a small library inline
function uuid() {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
    var r = Math.random() * 16 | 0;
    var v = c == 'x' ? r : (r & 0x3 | 0x8);
    return v.toString(16);
  });
}

// Use it
const id = uuid();
console.log("Generated ID:", id);

// Your task processing logic
const input = $.task.inputData;
// ...
```

### Option 3: External API Calls

Delegate complex logic to external services:

```javascript
(function() {
  // Call your own API that has full library support
  var response = http.post(
    "https://your-api.com/process-task",
    { "Content-Type": "application/json" },
    JSON.stringify($.task.inputData)
  );

  if (response.status === 200) {
    return {
      status: "COMPLETED",
      body: response.body
    };
  } else {
    return {
      status: "FAILED",
      body: { error: "API call failed" }
    };
  }
})();
```

### Option 4: Pre-process Data in Go

If you need heavy dependencies, you can:

1. Create a custom build of the worker with additional Go functions injected
2. Add your own utility functions in `injectUtilities()` in `cmd/worker.go`
3. Rebuild the CLI

Example - Adding a custom function:

```go
// In cmd/worker.go, add to injectUtilities()
vm.Set("myCustomFunction", func(data string) string {
    // Your Go code here using any Go library
    // Example: use a JSON schema validator, XML parser, etc.
    return processedData
})
```

Then use in JavaScript:
```javascript
const result = myCustomFunction($.task.inputData.xmlData);
```

### Option 5: Multiple Script Files (Workaround)

Load common code into your worker file:

```javascript
// common.js - Your library code
function helpers() {
  return {
    validateEmail: function(email) {
      return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);
    },
    formatDate: function(timestamp) {
      return new Date(timestamp).toISOString();
    }
  };
}

// worker.js - Copy common.js content at the top
function helpers() { /* ... */ }

// Use it
const h = helpers();
const isValid = h.validateEmail($.task.inputData.email);
```

## Complete Example: Real-World Worker

```javascript
// worker.js - Order processing with HTTP and crypto

// Get task data
const task = $.task;
const input = task.inputData;

console.log("Processing order:", input.orderId);

// Validate input
if (!input.orderId || !input.customerId) {
  return {
    status: "FAILED",
    body: { error: "Missing required fields" }
  };
}

// Generate idempotency key
const idempotencyKey = crypto.sha256(
  input.orderId + "-" + task.workflowInstanceId
);

// Fetch customer data
const customerResponse = http.get(
  "https://api.customers.com/customers/" + input.customerId,
  {
    "Authorization": "Bearer " + util.env("CUSTOMER_API_KEY"),
    "X-Idempotency-Key": idempotencyKey
  }
);

if (customerResponse.error) {
  console.error("Failed to fetch customer:", customerResponse.error);
  return {
    status: "FAILED",
    body: { error: "Customer API unavailable" }
  };
}

const customer = customerResponse.body;

// Calculate order total
const total = input.items.reduce(function(sum, item) {
  return sum + (item.price * item.quantity);
}, 0);

// Apply customer discount
const discount = customer.discountPercent || 0;
const finalTotal = total * (1 - discount / 100);

// Create order in payment system
const paymentResponse = http.post(
  "https://api.payments.com/orders",
  {
    "Authorization": "Bearer " + util.env("PAYMENT_API_KEY"),
    "Content-Type": "application/json",
    "X-Idempotency-Key": idempotencyKey
  },
  JSON.stringify({
    orderId: input.orderId,
    customerId: input.customerId,
    amount: finalTotal,
    currency: "USD"
  })
);

if (paymentResponse.error || paymentResponse.status !== 201) {
  console.error("Payment creation failed");
  return {
    status: "FAILED",
    body: {
      error: "Payment processing failed",
      details: paymentResponse.error || paymentResponse.text
    }
  };
}

// Success
console.log("Order processed successfully:", input.orderId);

return {
  status: "COMPLETED",
  body: {
    orderId: input.orderId,
    customerId: input.customerId,
    total: finalTotal,
    discount: discount,
    paymentId: paymentResponse.body.paymentId,
    processedAt: new Date().toISOString(),
    idempotencyKey: idempotencyKey
  }
};
```

## Adding More Utilities

If you need additional utilities, you can extend the worker by:

1. **Editing `cmd/worker.go`**
2. **Adding functions to `injectUtilities()`**
3. **Rebuilding the CLI**

Example - Add XML parsing:
```go
// In cmd/worker.go
import "encoding/xml"

// In injectUtilities()
vm.Set("parseXML", func(xmlStr string) map[string]interface{} {
    var result map[string]interface{}
    err := xml.Unmarshal([]byte(xmlStr), &result)
    if err != nil {
        return map[string]interface{}{"error": err.Error()}
    }
    return result
})
```

Then use in JavaScript:
```javascript
const xmlData = $.task.inputData.xmlString;
const parsed = parseXML(xmlData);
if (parsed.error) {
  console.error("XML parsing failed:", parsed.error);
}
```

## Summary

| Need | Solution |
|------|----------|
| HTTP calls | Use built-in `http` object |
| Hashing/encoding | Use built-in `crypto` object |
| String manipulation | Use built-in `str` object |
| Environment variables | Use `util.env()` |
| Small utilities | Include JavaScript code inline |
| Complex libraries | Call external APIs via `http` |
| Custom Go functions | Modify `injectUtilities()` and rebuild |

The JavaScript worker is designed for lightweight task processing with HTTP integration. For heavy processing or complex dependencies, consider calling external services that have full library support.
