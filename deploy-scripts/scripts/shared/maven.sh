#!/bin/bash
# Shared Maven functions for OCD scripts

# =============================================================================
# MAVEN SETTINGS MANAGEMENT
# =============================================================================

get_maven_settings() {
    write_colored_output "Configuring Maven settings..." "blue"

    if [[ "$RUNTIME_ENV" == "WSL" ]]; then
        local user=$(ls -1 /mnt/c/Users | grep -vE '^(Public|Default|desktop.ini|Default\ User|ADMINI~1|All\ Users)$' | head -n 1)

        if [[ -z "$user" ]]; then
            write_colored_output "Error: Could not determine Windows username" "red"
            exit 1
        fi

        if [[ ! -f "/mnt/c/Users/$user/.m2/settings.xml" ]]; then
            write_colored_output "Error: Maven settings.xml not found at /mnt/c/Users/$user/.m2/settings.xml" "red"
            exit 1
        fi

        WINDOWS_USER="$user"
        MAVEN_SETTINGS_PATH="C:\\Users\\$user\\.m2\\settings.xml"
        write_colored_output "Found Maven settings at: $MAVEN_SETTINGS_PATH" "green"
    elif [[ "$RUNTIME_ENV" == "MACOS" || "$RUNTIME_ENV" == "LINUX" ]]; then
        # Use standard Unix/Linux Maven settings path
        local settings_path="$HOME/.m2/settings.xml"

        if [[ ! -f "$settings_path" ]]; then
            write_colored_output "Warning: Maven settings.xml not found at $settings_path" "yellow"
            write_colored_output "Maven will use default configuration" "yellow"
            MAVEN_SETTINGS_PATH=""
            return 0
        fi

        MAVEN_SETTINGS_PATH="$settings_path"
        write_colored_output "Found Maven settings at: $MAVEN_SETTINGS_PATH" "green"
    else
        # Windows (Git Bash or similar)
        local user=$(ls -1 /c/Users | grep -vE '^(Public|Default|desktop.ini|Default\ User|ADMINI~1|All\ Users)$' | head -n 1)

        if [[ -z "$user" ]]; then
            write_colored_output "Error: Could not determine Windows username" "red"
            exit 1
        fi

        if [[ ! -f "/c/Users/$user/.m2/settings.xml" ]]; then
            write_colored_output "Error: Maven settings.xml not found at /c/Users/$user/.m2/settings.xml" "red"
            exit 1
        fi

        WINDOWS_USER="$user"
        MAVEN_SETTINGS_PATH="C:\\Users\\$user\\.m2\\settings.xml"
        write_colored_output "Found Maven settings at: $MAVEN_SETTINGS_PATH" "green"
    fi
}

# =============================================================================
# DOCKER AND REGISTRY FUNCTIONS
# =============================================================================

get_registry_and_tag_from_settings() {
    local settings_file_path=""

    if [[ "$RUNTIME_ENV" == "WSL" ]]; then
        settings_file_path="/mnt/c/Users/$WINDOWS_USER/.m2/settings.xml"
    elif [[ "$RUNTIME_ENV" == "MACOS" || "$RUNTIME_ENV" == "LINUX" ]]; then
        settings_file_path="$MAVEN_SETTINGS_PATH"

        # If no Maven settings available, return defaults or fail gracefully
        if [[ -z "$settings_file_path" || ! -f "$settings_file_path" ]]; then
            write_colored_output "Warning: No Maven settings available, using default Docker registry settings" "yellow"
            echo "docker.io|latest"
            return 0
        fi
    else
        settings_file_path="/c/Users/$WINDOWS_USER/.m2/settings.xml"
    fi

    local push_registry=$(grep -o '<docker\.push\.registry>[^<]*</docker\.push\.registry>' "$settings_file_path" | sed 's/<[^>]*>//g')
    local current_docker_tag=$(grep -o '<docker\.tag3>[^<]*</docker\.tag3>' "$settings_file_path" | sed 's/<[^>]*>//g')

    if [[ -z "$push_registry" ]]; then
        write_colored_output "Error: Could not find docker.push.registry in Maven settings" "red"
        return 1
    fi

    if [[ -z "$current_docker_tag" ]]; then
        write_colored_output "Error: Could not find docker.tag3 in Maven settings" "red"
        return 1
    fi

    echo "$push_registry|$current_docker_tag"
    return 0
}

# =============================================================================
# MAVEN BUILD FUNCTIONS
# =============================================================================

build_with_maven() {
    local build_dir="$1"
    local service_name="$2"
    
    write_colored_output "Building $service_name..." "blue"
    
    if [[ ! -d "$build_dir" ]]; then
        write_colored_output "Error: Build directory $build_dir not found" "red"
        return 1
    fi
    
    # Build Maven command
    local mvn_command="mvn clean install -DskipTests"
    if [[ -n "$MAVEN_SETTINGS_PATH" ]]; then
        mvn_command="$mvn_command -s '$MAVEN_SETTINGS_PATH'"
    fi
    
    log_command "$mvn_command"
    
    # Change to build directory and run Maven
    if [[ "$VERBOSE" == "true" ]]; then
        if (cd "$build_dir" && $mvn_command); then
            write_colored_output "Build completed successfully for $service_name" "green"
            return 0
        else
            write_colored_output "Maven build failed for $service_name" "red"
            return 1
        fi
    else
        if (cd "$build_dir" && $mvn_command > /dev/null 2>&1); then
            write_colored_output "Build completed successfully for $service_name" "green"
            return 0
        else
            write_colored_output "Maven build failed for $service_name" "red"
            # Run again without -q to show error details
            write_colored_output "Error details:" "red"
            (cd "$build_dir" && $mvn_command) 2>&1 | tail -20
            return 1
        fi
    fi
}


