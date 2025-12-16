import java.io.*;
import java.util.*;
import java.util.stream.*;

/**
 * Simple stdio worker example for Orkes Conductor CLI.
 *
 * This worker reads a task from stdin, processes it, and returns a result to stdout.
 * Demonstrates basic JSON handling without external dependencies.
 */
public class SimpleWorker {

    public static void main(String[] args) {
        try {
            // Read task from stdin
            BufferedReader reader = new BufferedReader(new InputStreamReader(System.in));
            String taskJson = reader.lines().collect(Collectors.joining("\n"));

            // Simple JSON parsing (in production, use a library like Gson or Jackson)
            String name = extractValue(taskJson, "name", "World");

            // Get task metadata from JSON
            String taskId = extractValue(taskJson, "taskId", "unknown");
            String workflowId = extractValue(taskJson, "workflowInstanceId", "unknown");
            String taskType = extractValue(taskJson, "taskType", "unknown");

            // Process the task
            String message = "Hello, " + name + "!";

            // Build result JSON
            StringBuilder result = new StringBuilder();
            result.append("{");
            result.append("\"status\":\"COMPLETED\",");
            result.append("\"output\":{");
            result.append("\"message\":\"").append(escapeJson(message)).append("\",");
            result.append("\"taskId\":\"").append(taskId).append("\",");
            result.append("\"workflowId\":\"").append(workflowId).append("\"");
            result.append("},");
            result.append("\"logs\":[");
            result.append("\"Processing task " + taskId + " of type " + taskType + "\",");
            result.append("\"Workflow: " + workflowId + "\",");
            result.append("\"Generated greeting for " + name + "\"");
            result.append("]");
            result.append("}");

            // Output result to stdout
            System.out.println(result.toString());

        } catch (Exception e) {
            // Return failure result on error
            String errorResult = String.format(
                "{\"status\":\"FAILED\",\"reason\":\"%s\",\"logs\":[\"Error processing task: %s\"]}",
                escapeJson(e.getMessage()),
                escapeJson(e.getMessage())
            );
            System.out.println(errorResult);
        }
    }

    /**
     * Simple JSON value extraction (for demonstration purposes).
     * In production, use a proper JSON library like Gson or Jackson.
     */
    private static String extractValue(String json, String key, String defaultValue) {
        String pattern = "\"" + key + "\"\\s*:\\s*\"([^\"]+)\"";
        java.util.regex.Pattern p = java.util.regex.Pattern.compile(pattern);
        java.util.regex.Matcher m = p.matcher(json);
        if (m.find()) {
            return m.group(1);
        }
        return defaultValue;
    }

    /**
     * Escape JSON string values.
     */
    private static String escapeJson(String value) {
        if (value == null) return "";
        return value
            .replace("\\", "\\\\")
            .replace("\"", "\\\"")
            .replace("\n", "\\n")
            .replace("\r", "\\r")
            .replace("\t", "\\t");
    }
}
