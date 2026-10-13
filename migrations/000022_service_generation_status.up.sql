ALTER TABLE tgt_service
  ADD COLUMN generation_status TEXT NOT NULL DEFAULT 'pending'
    CHECK (generation_status IN ('pending', 'confirmed'));
