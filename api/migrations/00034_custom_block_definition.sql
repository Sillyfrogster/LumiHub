-- +goose Up
-- The definition was named while the interface still said section. The domain
-- and the code both say block now, so the stored name follows them.
update asset_blocks set definition = 'custom_block' where definition = 'custom_section';

-- +goose Down
update asset_blocks set definition = 'custom_section' where definition = 'custom_block';
