-- Add display_name column to users table
ALTER TABLE users ADD COLUMN display_name VARCHAR(255) DEFAULT NULL AFTER email;
