-- Add ip_address column to clicks table
ALTER TABLE clicks ADD COLUMN ip_address VARCHAR(45) AFTER ip_hash;

-- Create index for potential future queries by IP
CREATE INDEX idx_clicks_ip_address ON clicks(ip_address);
