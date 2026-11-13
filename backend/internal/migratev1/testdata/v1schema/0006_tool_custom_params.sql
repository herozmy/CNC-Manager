ALTER TABLE program_tool
ADD COLUMN custom_params_json TEXT NOT NULL DEFAULT '[]';
