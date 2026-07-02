ALTER TABLE pastes DROP CONSTRAINT key_length;
ALTER TABLE pastes ADD CONSTRAINT key_length CHECK (char_length(key) BETWEEN 4 AND 32);
